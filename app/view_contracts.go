package app

import (
	"context"
	"errors"
	"fmt"
	"text/template"

	"fivebit.co.uk/spacetraders/prompt"
	"fivebit.co.uk/spacetraders/state"
)

var contractShortTemplate = template.Must(template.New("contract_short").Parse(`
{{- .Contract.Type}}[{{.Contract.FactionSymbol}}]
{{- if .Contract.Terms.Deliver}}({{range $i, $d := .Contract.Terms.Deliver}}{{if gt $i 0}}, {{end}}{{$d.TradeSymbol}}{{end}}){{end}} (
{{- if .Contract.Fulfilled}}fulfilled{{else}}{{if .Contract.Accepted}}{{if not .Active}}no {{end}}ships assigned{{else}}accept by {{.Contract.DeadlineToAccept}}{{end}}, due by {{.Contract.Terms.Deadline}}{{end}})`))

var contractFullTemplate = template.Must(template.New("contract_full").Funcs(template.FuncMap{
	"add": func(a, b int32) int32 { return a + b },
}).Parse(`
ID: {{.Contract.Id}}
Faction: {{.Contract.FactionSymbol}}
Type: {{.Contract.Type}}
Status: {{if .Contract.Fulfilled}}fulfilled{{else}}{{if .Contract.Accepted}}{{if not .Active}}no {{end}}ships assigned{{else}}accept by {{.Contract.DeadlineToAccept}}{{end}}, due by {{.Contract.Terms.Deadline}}{{end}}
Payment: {{.Contract.Terms.Payment.OnAccepted}} advance, {{.Contract.Terms.Payment.OnFulfilled}} fulfilled, {{add .Contract.Terms.Payment.OnAccepted .Contract.Terms.Payment.OnFulfilled}} total
Deliveries:{{range .Contract.Terms.Deliver}}
  {{.TradeSymbol}} to {{.DestinationSymbol}} ({{.UnitsFulfilled}} of {{.UnitsRequired}} delivered){{end}}{{if .Active}}
Assigned Ships:{{range .Ships}}
  {{.Registration.Name}} ({{.Registration.Role}}), {{.Nav.Status}} at {{.Nav.WaypointSymbol}}{{if ne .Nav.WaypointSymbol .Nav.Route.Destination.Symbol}} destination {{.Nav.Route.Destination.Symbol}}{{end}}, cargo {{.Cargo.Units}}/{{.Cargo.Capacity}}{{end}}{{end}}
`))

func viewContracts(ctx context.Context, app *App) error {
	return listContracts(ctx, app, 1)
}

func listContracts(ctx context.Context, app *App, page int32) error {
	resp, _, err := app.client.ContractsApi.GetContracts(ctx).Page(page).Limit(10).Execute()
	if err != nil {
		return err
	}

	var items []prompt.MenuItem
	if page > 1 {
		items = append(items, prompt.MenuItem{
			Label: "Previous page",
			Fn: func() error {
				return listContracts(ctx, app, page-1)
			},
		})
	}
	for _, c := range resp.Data {
		ac := app.augmentContract(c)
		label, err := stringTemplate(contractShortTemplate, ac)
		if err != nil {
			return err
		}
		items = append(items, prompt.MenuItem{
			Label: label,
			Fn: func() error {
				return viewContract(ctx, app, ac)
			},
			Loop: true,
		})
	}
	if page*10 < resp.Meta.Total {
		items = append(items, prompt.MenuItem{
			Label: "Next page",
			Fn: func() error {
				return listContracts(ctx, app, page+1)
			},
		})
	}
	items = append(items, prompt.MenuItemBack)

	return prompt.Menu("Select contract", items)
}

func viewContract(ctx context.Context, app *App, ac *AugmentedContract) error {
	for {
		if err := printTemplate(contractFullTemplate, ac); err != nil {
			return err
		}

		if ac.Contract.Fulfilled {
			return nil
		}

		var items []prompt.MenuItem

		if !ac.Contract.Accepted {
			items = append(items, prompt.MenuItem{
				Label: "Accept and assign ships",
				Fn: func() error {
					return acceptAndAssign(ctx, app, ac)
				},
			})
		} else {
			items = append(items, prompt.MenuItem{
				Label: "Assign ships",
				Fn: func() error {
					return assignShips(ctx, app, ac)
				},
			})
		}
		if ac.Active() {
			items = append(items, prompt.MenuItem{
				Label: "Unassign ships",
				Fn: func() error {
					fmt.Println("TODO")
					return nil
				},
			})
		}
		items = append(items, prompt.MenuItemBackErr)

		if err := prompt.Menu("Select action", items); err != nil {
			if errors.Is(err, prompt.ErrGoBack) {
				break
			} else {
				return err
			}
		}
	}
	return nil
}

func acceptAndAssign(ctx context.Context, app *App, ac *AugmentedContract) error {
	resp, _, err := app.client.ContractsApi.AcceptContract(ctx, ac.Contract.Id).Execute()
	if err != nil {
		return err
	}
	ac.Contract = resp.Data.Contract
	app.agent = resp.Data.Agent
	return assignShips(ctx, app, ac)
}

func assignShips(ctx context.Context, app *App, ac *AugmentedContract) error {
	var items []prompt.MenuItem
	for shipID := range app.ships {
		as := app.augmentShip(shipID)
		if as.Contract() != nil {
			continue
		}
		label, err := stringTemplate(shipShortTemplate, as)
		if err != nil {
			return err
		}
		items = append(items, prompt.MenuItem{
			Label: label,
			Fn: func() error {
				return assignShip(ctx, app, ac, as)
			},
			Loop: true,
		})
	}
	items = append(items, prompt.MenuItem{
		Label: "Buy new ship",
		Fn: func() error {
			return buyShipAndAssign(ctx, app, ac)
		},
		Loop: true,
	})
	items = append(items, prompt.MenuItemBack)
	return prompt.Menu("Select ship", items)
}

func assignShip(ctx context.Context, app *App, ac *AugmentedContract, as *AugmentedShip) error {
	if err := app.state.Update(func(ms state.MutableState) error {
		ms.AssignShip(ac.Contract.Id, as.shipID)
		return nil
	}); err != nil {
		return err
	}
	ac.Ships = append(ac.Ships, as.Ship())
	as.contractID = ac.Contract.Id
	_, err := app.shipActivity(ctx, as)
	return err
}

func buyShipAndAssign(ctx context.Context, app *App, ac *AugmentedContract) error {
	ship, err := buyAndReturnShip(ctx, app)
	if err != nil {
		return err
	}
	if ship == nil {
		return nil
	}
	return assignShip(ctx, app, ac, ship)
}

func unassignShip(ctx context.Context, app *App, ac *AugmentedContract, as *AugmentedShip) error {
	if err := app.state.Update(func(ms state.MutableState) error {
		ms.UnassignShip(ac.Contract.Id, as.shipID)
		return nil
	}); err != nil {
		return err
	}
	*ac = *app.augmentContract(ac.Contract)
	as.contractID = ""
	return nil
}
