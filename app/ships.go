package app

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"fivebit.co.uk/spacetraders/api"
)

var shipShortTemplate = template.Must(template.New("ship_short").Parse(`
{{- .Ship.Registration.Name}} ({{.Ship.Registration.Role}}{{if .Contract}}, assigned{{end}}),
{{- if eq .Ship.Nav.Status "IN_TRANSIT"}} in transit ({{.Ship.Nav.Route.Destination.Symbol}}, {{.Ship.Nav.Route.Arrival}})
{{- else}} {{.Ship.Nav.Status}} at {{.Ship.Nav.WaypointSymbol}}{{end -}}
, cargo {{.Ship.Cargo.Units}}/{{.Ship.Cargo.Capacity}}`))

var shipFullTemplate = template.Must(template.New("ship_full").Parse(`
ID: {{.Ship.Symbol}}
Name: {{.Ship.Registration.Name}}
Role: {{.Ship.Registration.Role}}{{if .Contract}}
Contract: {{.Contract.Type}}[{{.Contract.FactionSymbol}}]
	{{- if .Contract.Terms.Deliver}}({{range $i, $d := .Contract.Terms.Deliver}}{{if gt $i 0}}, {{end}}{{$d.TradeSymbol}}{{end}}){{end}}{{end}}
Navigation status:
	{{- if eq .Ship.Nav.Status "IN_TRANSIT"}} in transit to {{.Ship.Nav.Route.Destination.Symbol}}, arriving {{.Ship.Nav.Route.Arrival}}
	{{- else}} {{.Ship.Nav.Status}} at {{.Ship.Nav.WaypointSymbol}}{{end}}
Fuel: {{.Ship.Fuel.Current}}/{{.Ship.Fuel.Capacity}}
Cargo: {{.Ship.Cargo.Units}}/{{.Ship.Cargo.Capacity}}{{range .Ship.Cargo.Inventory}}
  {{.Name}} ({{.Symbol}}): {{.Units}}{{end}}
`))

type AugmentedShip struct {
	app        *App
	shipID     string
	contractID string
}

func (a *App) augmentShip(shipID string) *AugmentedShip {
	return &AugmentedShip{
		app:        a,
		shipID:     shipID,
		contractID: a.state.AssignedContract(shipID),
	}
}

func (as *AugmentedShip) Ship() api.Ship {
	s, ok := as.app.ships[as.shipID]
	if !ok {
		panic(fmt.Errorf("ship %s not found", as.shipID))
	}
	return s
}

func (as *AugmentedShip) Contract() *api.Contract {
	if as.contractID == "" {
		return nil
	}
	c, ok := as.app.activeContracts[as.contractID]
	if !ok {
		panic(fmt.Errorf("contract %s not found", as.contractID))
	}
	return &c
}

func (as *AugmentedShip) Refresh(ctx context.Context) error {
	resp, _, err := as.app.client.FleetApi.GetMyShip(ctx, as.shipID).Execute()
	if err != nil {
		return err
	}
	as.app.ships[as.shipID] = resp.Data
	return nil
}

func (as *AugmentedShip) Dock(ctx context.Context) error {
	resp, _, err := as.app.client.FleetApi.DockShip(ctx, as.shipID).Execute()
	if err != nil {
		return err
	}
	ship := as.app.ships[as.shipID]
	ship.Nav = resp.Data.Nav
	as.app.ships[as.shipID] = ship
	return nil
}

func (as *AugmentedShip) Orbit(ctx context.Context) error {
	resp, _, err := as.app.client.FleetApi.OrbitShip(ctx, as.shipID).Execute()
	if err != nil {
		return err
	}
	ship := as.app.ships[as.shipID]
	ship.Nav = resp.Data.Nav
	as.app.ships[as.shipID] = ship
	return nil
}

func (as *AugmentedShip) TryRefuel(ctx context.Context) error {
	if as.Ship().Nav.Status != api.SHIPNAVSTATUS_DOCKED {
		if err := as.Dock(ctx); err != nil {
			return err
		}
	}
	resp, _, err := as.app.client.FleetApi.RefuelShip(ctx, as.shipID).Execute()
	if err != nil {
		fmt.Printf("Failed to refuel ship: %v", err)
		return nil
	}
	ship := as.app.ships[as.shipID]
	ship.Fuel = resp.Data.Fuel
	as.app.ships[as.shipID] = ship
	as.app.agent = resp.Data.Agent
	return nil
}

func (as *AugmentedShip) DeliverGoods(ctx context.Context, tradeSymbol string, units int32) error {
	if as.Ship().Nav.Status != api.SHIPNAVSTATUS_DOCKED {
		if err := as.Dock(ctx); err != nil {
			return err
		}
	}
	resp, _, err := as.app.client.ContractsApi.DeliverContract(ctx, as.contractID).DeliverContractRequest(api.DeliverContractRequest{
		ShipSymbol:  as.shipID,
		TradeSymbol: tradeSymbol,
		Units:       units,
	}).Execute()
	if err != nil {
		return err
	}
	ship := as.app.ships[as.shipID]
	ship.Cargo = resp.Data.Cargo
	as.app.ships[as.shipID] = ship
	as.app.activeContracts[as.contractID] = resp.Data.Contract
	return nil
}

func (as *AugmentedShip) SellCargo(ctx context.Context, tradeSymbol string, units int32) error {
	if as.Ship().Nav.Status != api.SHIPNAVSTATUS_DOCKED {
		if err := as.Dock(ctx); err != nil {
			return err
		}
	}
	resp, _, err := as.app.client.FleetApi.SellCargo(ctx, as.shipID).SellCargoRequest(api.SellCargoRequest{
		Symbol: tradeSymbol,
		Units:  units,
	}).Execute()
	if err != nil {
		return err
	}
	ship := as.app.ships[as.shipID]
	ship.Cargo = resp.Data.Cargo
	as.app.ships[as.shipID] = ship
	as.app.agent = resp.Data.Agent
	return nil
}

func (as *AugmentedShip) Extract(ctx context.Context, symbol string) (time.Time, error) {
	if as.Ship().Nav.Status != api.SHIPNAVSTATUS_IN_ORBIT {
		if err := as.Orbit(ctx); err != nil {
			return time.Time{}, err
		}
	}
	req := api.ExtractResourcesRequest{}
	if symbol != "" {
		req.Survey = as.app.getSurvey(as.Ship().Nav.WaypointSymbol, symbol)
		if req.Survey != nil {
			fmt.Printf("Using survey: %s\n", formatSurvey(*req.Survey))
		}
	}
	resp, httpResp, err := as.app.client.FleetApi.ExtractResources(ctx, as.shipID).ExtractResourcesRequest(req).Execute()
	if err != nil {
		if httpResp.StatusCode == 409 {
			apiError, err := getAPIError(err)
			if err != nil {
				return time.Time{}, err
			}
			if apiError.Code != 4000 {
				return time.Time{}, fmt.Errorf("Unexpected 409 response: %d - %s", apiError.Code, apiError.Message)
			}
			cd := &api.Cooldown{}
			if err := apiError.decodeData("cooldown", cd); err != nil {
				return time.Time{}, err
			}
			fmt.Printf("Still on cooldown; %d of %d seconds remaining\n", cd.RemainingSeconds, cd.TotalSeconds)
			return cd.GetExpiration(), nil
		}
		return time.Time{}, err
	}
	ship := as.app.ships[as.shipID]
	ship.Cargo = resp.Data.Cargo
	as.app.ships[as.shipID] = ship
	return resp.Data.Cooldown.GetExpiration(), nil
}

func (as *AugmentedShip) TravelTo(ctx context.Context, waypoint string) (time.Time, error) {
	resp, _, err := as.app.client.FleetApi.NavigateShip(ctx, as.shipID).NavigateShipRequest(api.NavigateShipRequest{
		WaypointSymbol: waypoint,
	}).Execute()
	if err != nil {
		return time.Time{}, err
	}
	ship := as.app.ships[as.shipID]
	ship.Nav = resp.Data.Nav
	ship.Fuel = resp.Data.Fuel
	as.app.ships[as.shipID] = ship
	return resp.Data.Nav.Route.Arrival, nil
}

func (as *AugmentedShip) Survey(ctx context.Context) (time.Time, error) {
	if as.Ship().Nav.Status != api.SHIPNAVSTATUS_IN_ORBIT {
		if err := as.Orbit(ctx); err != nil {
			return time.Time{}, err
		}
	}
	resp, httpResp, err := as.app.client.FleetApi.CreateSurvey(ctx, as.shipID).Execute()
	if err != nil {
		if httpResp.StatusCode == 409 {
			apiError, err := getAPIError(err)
			if err != nil {
				return time.Time{}, err
			}
			if apiError.Code != 4000 {
				return time.Time{}, fmt.Errorf("Unexpected 409 response: %d - %s", apiError.Code, apiError.Message)
			}
			cd := &api.Cooldown{}
			if err := apiError.decodeData("cooldown", cd); err != nil {
				return time.Time{}, err
			}
			fmt.Printf("Still on cooldown; %d of %d seconds remaining\n", cd.RemainingSeconds, cd.TotalSeconds)
			return cd.GetExpiration(), nil
		}
		return time.Time{}, err
	}

	as.app.setSurveys(as.Ship().Nav.WaypointSymbol, resp.Data.Surveys)

	return resp.Data.Cooldown.GetExpiration(), nil
}

func (as *AugmentedShip) HasMount(mountPrefix string) bool {
	for _, m := range as.Ship().Mounts {
		if strings.HasPrefix(m.Symbol, mountPrefix) {
			return true
		}
	}
	return false
}
