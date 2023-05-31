package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"fivebit.co.uk/spacetraders/api"
)

var ErrContractFulfilled = errors.New("contract fulfilled")

var defaultActivityInterval = 10 * time.Second

func runActivityLoop(ctx context.Context, app *App) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	timer := time.NewTimer(0)
	for {
		select {
		case <-interrupt:
			timer.Stop()
			return nil
		case <-timer.C:
			readyTime, err := app.runActivity(ctx)
			if err != nil && !errors.Is(err, ErrContractFulfilled) {
				return err
			}
			if readyTime.IsZero() {
				fmt.Printf("Waiting %s for next round\n", defaultActivityInterval)
				timer.Reset(defaultActivityInterval)
			} else {
				fmt.Printf("Waiting until %s for next round\n", readyTime)
				timer.Reset(readyTime.Sub(time.Now()))
			}
		}
	}
}

func (a *App) runActivity(ctx context.Context) (time.Time, error) {
	nextReadyTime := time.Time{}
	hasFulfilledContract := false
	for _, c := range a.activeContracts {
		ac := a.augmentContract(c)
		for _, s := range ac.Ships {
			readyTime, err := a.shipActivityWrapper(ctx, a.augmentShip(s.Symbol))
			if err != nil {
				if errors.Is(err, ErrContractFulfilled) {
					hasFulfilledContract = true
					break
				} else {
					return time.Time{}, err
				}
			}
			if !readyTime.IsZero() && (nextReadyTime.IsZero() || readyTime.Before(nextReadyTime)) {
				nextReadyTime = readyTime
			}
		}
	}
	if hasFulfilledContract {
		return nextReadyTime, ErrContractFulfilled
	}
	return nextReadyTime, nil
}

func (a *App) shipActivityWrapper(ctx context.Context, as *AugmentedShip) (time.Time, error) {
	if readyTime := a.getReadyTime(as.shipID); !readyTime.IsZero() {
		return readyTime, nil
	}
	readyTime, err := a.shipActivity(ctx, as)
	if !readyTime.IsZero() {
		a.setReadyTime(as.shipID, readyTime)
	}
	return readyTime, err
}

func (a *App) shipActivity(ctx context.Context, as *AugmentedShip) (time.Time, error) {
	if as.Contract().Type != "PROCUREMENT" {
		return time.Time{}, fmt.Errorf("Unsupported contract type %s", as.Contract().Type)
	}
	switch as.Ship().Registration.Role {
	case api.SHIPROLE_EXCAVATOR:
		return a.procurementActivity(ctx, as)
	case api.SHIPROLE_COMMAND:
		return a.procurementActivity(ctx, as)
	default:
		return time.Time{}, fmt.Errorf("Unsupported ship role %s", as.Ship().Registration.Role)
	}
}

func (a *App) procurementActivity(ctx context.Context, as *AugmentedShip) (time.Time, error) {
	readyTime, err := a.checkShipTransit(ctx, as)
	if err != nil || !readyTime.IsZero() {
		return readyTime, err
	}

	waypointTraits, err := a.getCurrentWaypointTraits(ctx, as)
	if err != nil {
		return time.Time{}, err
	}

	if waypointTraits["MARKETPLACE"] && as.Ship().Fuel.Current < as.Ship().Fuel.Capacity {
		fmt.Printf("%s (%s) attempting to refuel\n", as.Ship().Registration.Name, as.Ship().Registration.Role)
		if err := as.TryRefuel(ctx); err != nil {
			return time.Time{}, err
		}
	}

	// Deliver contract cargo
	availableCargo := map[string]int32{}
	for _, c := range as.Ship().Cargo.Inventory {
		availableCargo[c.Symbol] = c.Units
	}

	var deliveriesAtCurrentWaypoint []api.ContractDeliverGood
	var deliveriesElsewhere []api.ContractDeliverGood

	for _, d := range as.Contract().Terms.Deliver {
		if d.UnitsFulfilled >= d.UnitsRequired {
			continue
		}
		if d.DestinationSymbol == as.Ship().Nav.WaypointSymbol {
			deliveriesAtCurrentWaypoint = append(deliveriesAtCurrentWaypoint, d)
		} else {
			deliveriesElsewhere = append(deliveriesElsewhere, d)
		}
	}

	cargoToDeliver := map[string]int32{}
	otherDeliveryLocations := map[string]bool{}
	materialsToObtain := map[string]bool{}

	for _, d := range deliveriesAtCurrentWaypoint {
		unitsAvailable, ok := availableCargo[d.TradeSymbol]
		if !ok {
			materialsToObtain[d.TradeSymbol] = true
			continue
		}
		unitsRequired := d.UnitsRequired - d.UnitsFulfilled
		unitsToDeliver := unitsRequired
		if unitsRequired >= unitsAvailable {
			if unitsRequired > unitsAvailable {
				materialsToObtain[d.TradeSymbol] = true
			}
			unitsToDeliver = unitsAvailable
			delete(availableCargo, d.TradeSymbol)
		} else {
			availableCargo[d.TradeSymbol] -= unitsToDeliver
		}
		cargoToDeliver[d.TradeSymbol] += unitsToDeliver
	}

	for _, d := range deliveriesElsewhere {
		unitsAvailable, ok := availableCargo[d.TradeSymbol]
		if !ok {
			materialsToObtain[d.TradeSymbol] = true
			continue
		}
		otherDeliveryLocations[d.DestinationSymbol] = true
		unitsRequired := d.UnitsRequired - d.UnitsFulfilled
		if unitsRequired >= unitsAvailable {
			if unitsRequired > unitsAvailable {
				materialsToObtain[d.TradeSymbol] = true
			}
			delete(availableCargo, d.TradeSymbol)
		} else {
			availableCargo[d.TradeSymbol] -= unitsRequired
		}
	}

	for symbol, units := range cargoToDeliver {
		fmt.Printf("%s (%s) delivering %d units of %s at %s\n", as.Ship().Registration.Name, as.Ship().Registration.Role, units, symbol, as.Ship().Nav.WaypointSymbol)
		if err := as.DeliverGoods(ctx, symbol, units); err != nil {
			return time.Time{}, err
		}
	}

	// Sell unnecessary cargo
	if waypointTraits["MARKETPLACE"] {
		for symbol, units := range availableCargo {
			if symbol == "ANTIMATTER" {
				continue
			}
			fmt.Printf("%s (%s) selling %d unneeded units of %s at %s\n", as.Ship().Registration.Name, as.Ship().Registration.Role, units, symbol, as.Ship().Nav.WaypointSymbol)
			if err := as.SellCargo(ctx, symbol, units); err != nil {
				return time.Time{}, err
			}
		}
	}

	if len(otherDeliveryLocations) == 0 && len(materialsToObtain) == 0 {
		// We don't need to deliver anything elsewhere, and we don't need to obtain more materials, so
		// the contract is complete. The ship then has no assigned contract, and thus has nothing to do.
		// Note: the fulfillContract method prints to stdout
		a.fulfillContract(ctx, as.contractID)
		return time.Time{}, ErrContractFulfilled
	}

	// If this ship can survey, and we don't have high quality surveys for all materials we want, do a
	// survey. Note that surveying and extraction use the same cooldown (i.e. we can't survey and then
	// immediately extract resources)
	if as.HasMount("MOUNT_SURVEYOR") {
		shouldSurvey := false
		for material := range materialsToObtain {
			survey := a.getSurvey(as.Ship().Nav.WaypointSymbol, material)
			if survey == nil || mineralFractions(*survey)[material] < 0.2 {
				shouldSurvey = true
			}
		}
		if shouldSurvey {
			fmt.Printf("%s (%s) surveying at %s\n", as.Ship().Registration.Name, as.Ship().Registration.Role, as.Ship().Nav.WaypointSymbol)
			return as.Survey(ctx)
		}
	}

	// If currently at a mining location, more materials are required, and there is space in the cargo
	// hold, do some mining
	if waypointTraits["MINERAL_DEPOSITS"] && len(materialsToObtain) > 0 && float64(as.Ship().Cargo.Units)/float64(as.Ship().Cargo.Capacity) < 0.85 {
		for material := range materialsToObtain {
			fmt.Printf("%s (%s) extracting %s at %s\n", as.Ship().Registration.Name, as.Ship().Registration.Role, material, as.Ship().Nav.WaypointSymbol)
			return as.Extract(ctx, material)
		}
	}

	// If we have cargo to deliver elsewhere, go there.
	// TODO: Choose location in a clever way instead of randomly/arbitrarily
	for location := range otherDeliveryLocations {
		fmt.Printf("%s (%s) travelling to %s to deliver goods\n", as.Ship().Registration.Name, as.Ship().Registration.Role, location)
		return as.TravelTo(ctx, location)
	}

	// Otherwise, we need to find somewhere to obtain minerals
	wps, err := a.getWaypoints(ctx, as.Ship().Nav.SystemSymbol)
	if err != nil {
		return time.Time{}, err
	}
	for _, wp := range wps {
		for _, t := range wp.Traits {
			if t.Symbol == "MINERAL_DEPOSITS" {
				fmt.Printf("%s (%s) travelling to %s to extract resources\n", as.Ship().Registration.Name, as.Ship().Registration.Role, as.Ship().Nav.WaypointSymbol)
				return as.TravelTo(ctx, wp.Symbol)
			}
		}
	}

	return time.Time{}, errors.New("did not find suitable location to go mining")
}

func (a *App) checkShipTransit(ctx context.Context, as *AugmentedShip) (time.Time, error) {
	if as.Ship().Nav.Status == api.SHIPNAVSTATUS_IN_TRANSIT {
		if as.Ship().Nav.Route.Arrival.Before(time.Now()) {
			if err := as.Refresh(ctx); err != nil {
				return time.Time{}, err
			}
			if as.Ship().Nav.Status == api.SHIPNAVSTATUS_IN_TRANSIT {
				return time.Time{}, errors.New("Ship still in transit after arrival time")
			}
		} else {
			fmt.Printf("%s (%s) still in transit to %s, arriving %s\n", as.Ship().Registration.Name, as.Ship().Registration.Role, as.Ship().Nav.Route.Destination.Symbol, as.Ship().Nav.Route.Arrival)
			return as.Ship().Nav.Route.Arrival, nil
		}
	}
	return time.Time{}, nil
}

func (a *App) getCurrentWaypointTraits(ctx context.Context, as *AugmentedShip) (map[string]bool, error) {
	wp, err := a.getWaypoint(ctx, as.Ship().Nav.SystemSymbol, as.Ship().Nav.WaypointSymbol)
	if err != nil {
		return nil, err
	}
	waypointTraits := map[string]bool{}
	for _, t := range wp.Traits {
		waypointTraits[t.Symbol] = true
	}
	return waypointTraits, nil
}
