package app

import (
	"context"
	"fmt"
	"time"

	"fivebit.co.uk/spacetraders/api"
)

func (a *App) loadData(ctx context.Context) error {
	if err := a.loadAgent(ctx); err != nil {
		return err
	}
	if err := a.loadShips(ctx); err != nil {
		return err
	}
	if err := a.loadContracts(ctx); err != nil {
		return err
	}
	if err := a.loadWaypoints(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) loadAgent(ctx context.Context) error {
	resp, _, err := a.client.AgentsApi.GetMyAgent(ctx).Execute()
	if err != nil {
		return err
	}
	a.agent = resp.Data
	return nil
}

func (a *App) loadShips(ctx context.Context) error {
	page := int32(1)
	ships := map[string]api.Ship{}
	for {
		resp, _, err := a.client.FleetApi.GetMyShips(ctx).Page(page).Limit(20).Execute()
		if err != nil {
			return err
		}
		for _, ship := range resp.Data {
			ships[ship.Symbol] = ship
		}
		if page*20 < resp.Meta.Total {
			page++
		} else {
			break
		}
	}
	a.ships = ships
	return nil
}

func (a *App) loadContracts(ctx context.Context) error {
	contracts := map[string]api.Contract{}
	for _, contractID := range a.state.ActiveContracts() {
		resp, _, err := a.client.ContractsApi.GetContract(ctx, contractID).Execute()
		if err != nil {
			return err
		}
		contracts[resp.Data.Id] = resp.Data
	}
	a.activeContracts = contracts
	return nil
}

func (a *App) loadWaypoints(ctx context.Context) error {
	systems := map[string]bool{}
	for _, ship := range a.ships {
		systems[ship.Nav.SystemSymbol] = true
	}
	waypointsBySystem := map[string][]api.Waypoint{}
	for system := range systems {
		waypoints, err := a.fetchWaypoints(ctx, system)
		if err != nil {
			return err
		}
		waypointsBySystem[system] = waypoints
	}
	a.waypoints = waypointsBySystem
	return nil
}

func (a *App) getWaypoints(ctx context.Context, system string) ([]api.Waypoint, error) {
	if waypoints, ok := a.waypoints[system]; ok {
		return waypoints, nil
	}
	waypoints, err := a.fetchWaypoints(ctx, system)
	if err != nil {
		return nil, err
	}
	a.waypoints[system] = waypoints
	return waypoints, nil
}

func (a *App) getWaypoint(ctx context.Context, system string, waypoint string) (api.Waypoint, error) {
	waypoints, err := a.getWaypoints(ctx, system)
	if err != nil {
		return api.Waypoint{}, err
	}
	for _, wp := range waypoints {
		if wp.Symbol == waypoint {
			return wp, nil
		}
	}
	return api.Waypoint{}, fmt.Errorf("Waypoint %s not found in system %s", waypoint, system)
}

func (a *App) fetchWaypoints(ctx context.Context, system string) ([]api.Waypoint, error) {
	var waypoints []api.Waypoint
	page := int32(1)
	for {
		resp, _, err := a.client.SystemsApi.GetSystemWaypoints(ctx, system).Page(page).Limit(20).Execute()
		if err != nil {
			return nil, err
		}
		for _, wp := range resp.Data {
			waypoints = append(waypoints, wp)
		}
		if page*20 < resp.Meta.Total {
			page++
		} else {
			break
		}
	}
	return waypoints, nil
}

func (a *App) getSurvey(waypoint string, tradeSymbol string) *api.Survey {
	surveysForWaypoint, ok := a.surveys[waypoint]
	if !ok {
		return nil
	}
	survey, ok := surveysForWaypoint[tradeSymbol]
	if !ok {
		return nil
	}
	if survey.Expiration.Before(time.Now()) {
		delete(surveysForWaypoint, tradeSymbol)
		return nil
	}
	return survey
}

func (a *App) setSurveys(waypoint string, surveys []api.Survey) {
	surveysForWaypoint, ok := a.surveys[waypoint]
	if !ok {
		surveysForWaypoint = map[string]*api.Survey{}
		if a.surveys == nil {
			a.surveys = map[string]map[string]*api.Survey{}
		}
		a.surveys[waypoint] = surveysForWaypoint
	}
	for _, survey := range surveys {
		for symbol, fraction := range mineralFractions(survey) {
			other, ok := surveysForWaypoint[symbol]
			if !ok || other.Expiration.Sub(time.Now()) < 2 * time.Minute || fraction > mineralFractions(*other)[symbol] {
				surveysForWaypoint[symbol] = &survey
			}
		}
	}
}

func mineralCounts(survey api.Survey) (map[string]int32, int32) {
	mineralCount := map[string]int32{}
	totalCount := int32(0)
	for _, deposit := range survey.Deposits {
		mineralCount[deposit.Symbol]++
		totalCount++
	}
	return mineralCount, totalCount
}

func mineralFractions(survey api.Survey) map[string]float64 {
	mineralCount, totalCount := mineralCounts(survey)
	mineralFractions := map[string]float64{}
	for symbol, count := range mineralCount {
		mineralFractions[symbol] = float64(count)/float64(totalCount)
	}
	return mineralFractions
}

func (a *App) setReadyTime(shipID string, readyTime time.Time) {
	if a.shipReadyTimes == nil {
		a.shipReadyTimes = map[string]time.Time{}
	}
	a.shipReadyTimes[shipID] = readyTime
}

func (a *App) getReadyTime(shipID string) time.Time {
	readyTime := a.shipReadyTimes[shipID]
	if readyTime.IsZero() {
		return readyTime
	}
	if readyTime.Before(time.Now()) {
		a.setReadyTime(shipID, time.Time{})
		return time.Time{}
	}
	return readyTime
}
