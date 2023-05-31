package app

import (
	"context"
	"fmt"
	"text/template"

	"fivebit.co.uk/spacetraders/api"
	"fivebit.co.uk/spacetraders/prompt"
	"github.com/manifoldco/promptui"
)

var shipyardShipFullTemplate = template.Must(template.New("shipyard_ship_full").Parse(`
Name: {{.Name}}
Description: {{.Description}}
Price: {{.PurchasePrice}}C
Frame: {{.Frame.Name}} - {{.Frame.Description}}
  Module slots: {{.Frame.ModuleSlots}}
  Mounting points: {{.Frame.MountingPoints}}
  Fuel capacity: {{.Frame.FuelCapacity}}
  Requirements: {{.Frame.Requirements.Power}} power, {{.Frame.Requirements.Crew}} crew
Reactor: {{.Reactor.Name}} - {{.Reactor.Description}}
  Power output: {{.Reactor.PowerOutput}}
  Requirements: {{.Reactor.Requirements.Crew}} crew
Engine: {{.Engine.Name}} - {{.Engine.Description}}
  Speed: {{.Engine.Speed}}
  Requirements: {{.Engine.Requirements.Power}} power, {{.Engine.Requirements.Crew}} crew{{if .Modules}}
Modules:{{range .Modules}}
  {{.Name}} - {{.Description}}{{if .Capacity}}
    Capacity: {{.Capacity}}{{end}}
    Requirements: {{.Requirements.Power}} power, {{.Requirements.Crew}} crew, {{.Requirements.Slots}} slots{{end}}{{end}}{{if .Mounts}}
Mounts:{{range .Mounts}}
  {{.Name}} - {{.Description}}{{if .Strength}}
    Strength: {{.Strength}}{{end}}{{if .Deposits}}
    Deposits: {{range $i, $d := .Deposits}}{{if $i}}, {{end}}{{$d}}{{end}}{{end}}
    Requirements: {{.Requirements.Power}} power, {{.Requirements.Crew}} crew{{end}}{{end}}
`))

func buyShip(ctx context.Context, app *App) error {
	_, err := buyAndReturnShip(ctx, app)
	return err
}

func buyAndReturnShip(ctx context.Context, app *App) (*AugmentedShip, error) {
	shipLocations := map[string]bool{}
	for _, ship := range app.ships {
		shipLocations[ship.Nav.WaypointSymbol] = true
	}
	var shipyardsWithShips []string
	var shipyardsWithoutShips []string
	for _, waypoints := range app.waypoints {
		for _, waypoint := range waypoints {
			hasShipyard := false
			for _, trait := range waypoint.Traits {
				if trait.Symbol == "SHIPYARD" {
					hasShipyard = true
					break
				}
			}
			if !hasShipyard {
				continue
			}
			if shipLocations[waypoint.Symbol] {
				shipyardsWithShips = append(shipyardsWithShips, waypoint.Symbol)
			} else {
				shipyardsWithoutShips = append(shipyardsWithoutShips, waypoint.Symbol)
			}
		}
	}
	var items []string
	var symbols []string
	for _, sy := range shipyardsWithShips {
		items = append(items, sy+" (prices available)")
		symbols = append(symbols, sy)
	}
	for _, sy := range shipyardsWithoutShips {
		items = append(items, sy+" (prices not available)")
		symbols = append(symbols, sy)
	}
	items = append(items, "Cancel")

	i, _, err := (&promptui.Select{
		Label: "Select shipyard",
		Items: items,
	}).Run()
	if err != nil {
		return nil, err
	}
	if items[i] == "Cancel" {
		return nil, nil
	}

	return buyFromShipyard(ctx, app, symbols[i])
}

func buyFromShipyard(ctx context.Context, app *App, shipyard string) (*AugmentedShip, error) {
	wp, err := ParseWaypoint(shipyard)
	if err != nil {
		return nil, err
	}
	resp, _, err := app.client.SystemsApi.GetShipyard(ctx, wp.system.String(), wp.String()).Execute()
	if err != nil {
		return nil, err
	}
	var purchaseRequest *api.PurchaseShipRequest
	if len(resp.Data.Ships) > 0 {
		purchaseRequest, err = selectFromShips(shipyard, resp.Data.Ships)
	} else {
		purchaseRequest, err = selectFromShiptypes(shipyard, resp.Data.ShipTypes)
	}
	if err != nil {
		return nil, err
	}
	if purchaseRequest == nil {
		return nil, nil
	}

	buyResp, _, err := app.client.FleetApi.PurchaseShip(ctx).PurchaseShipRequest(*purchaseRequest).Execute()
	app.agent = buyResp.Data.Agent
	app.ships[buyResp.Data.Ship.Symbol] = buyResp.Data.Ship
	return app.augmentShip(buyResp.Data.Ship.Symbol), nil
}

func selectFromShips(shipyard string, ships []api.ShipyardShip) (*api.PurchaseShipRequest, error) {
	var shipItems []prompt.MenuItemWithResult[*api.ShipyardShip]
	for _, ship := range ships {
		ship := ship
		shipItems = append(shipItems, prompt.MenuItemWithResult[*api.ShipyardShip]{
			Label: fmt.Sprintf("%s (%d credits)", ship.Name, ship.PurchasePrice),
			Fn: func() (*api.ShipyardShip, error) {
				return &ship, nil
			},
		})
	}
	shipItems = append(shipItems, prompt.MenuItemWithResult[*api.ShipyardShip]{
		Label: "Cancel",
		Fn: func() (*api.ShipyardShip, error) {
			return nil, nil
		},
	})
	for {
		ship, err := prompt.MenuWithResult[*api.ShipyardShip]("Select ship", shipItems)
		if err != nil {
			return nil, err
		}
		if ship == nil {
			return nil, nil
		}
		printTemplate(shipyardShipFullTemplate, ship)
		doPurchase, err := prompt.MenuWithResult[bool]("Purchase ship?", []prompt.MenuItemWithResult[bool]{
			{
				Label: "Yes",
				Fn: func() (bool, error) {
					return true, nil
				},
			},
			{
				Label: "No",
				Fn: func() (bool, error) {
					return false, nil
				},
			},
		})
		if err != nil {
			return nil, err
		}
		if doPurchase {
			return api.NewPurchaseShipRequest(*ship.Type, shipyard), nil
		}
	}
}

func selectFromShiptypes(shipyard string, shipyardShipTypes []api.ShipyardShipTypesInner) (*api.PurchaseShipRequest, error) {
	var items []prompt.MenuItemWithResult[api.ShipType]
	for _, sst := range shipyardShipTypes {
		st, ok := sst.GetTypeOk()
		if !ok {
			continue
		}
		items = append(items, prompt.MenuItemWithResult[api.ShipType]{
			Label: string(*st),
			Fn: func() (api.ShipType, error) {
				return *st, nil
			},
		})
	}
	items = append(items, prompt.MenuItemWithResult[api.ShipType]{
		Label: "Cancel",
		Fn: func() (api.ShipType, error) {
			return "", nil
		},
	})
	shipType, err := prompt.MenuWithResult[api.ShipType]("Select ship type (no owned ship present at shipyard; details and prices not available)", items)
	if err != nil {
		return nil, err
	}
	if shipType == "" {
		return nil, nil
	}
	return api.NewPurchaseShipRequest(shipType, shipyard), nil
}
