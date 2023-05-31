package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"fivebit.co.uk/spacetraders/prompt"
)

func viewFleet(ctx context.Context, app *App) error {
	var items []prompt.MenuItem
	for shipID := range app.ships {
		as := app.augmentShip(shipID)
		label, err := stringTemplate(shipShortTemplate, as)
		if err != nil {
			return err
		}
		items = append(items, prompt.MenuItem{
			Label: label,
			Fn: func() error {
				return viewShip(ctx, app, as)
			},
			Loop: true,
		})
	}
	items = append(items, prompt.MenuItemBack)

	return prompt.Menu("Select ship", items)
}

func viewShip(ctx context.Context, app *App, as *AugmentedShip) error {
	for {
		if err := printTemplate(shipFullTemplate, as); err != nil {
			return err
		}

		var items []prompt.MenuItem

		if as.Contract() != nil {
			items = append(items, prompt.MenuItem{
				Label: "Unassign from contract",
				Fn: func() error {
					return unassignShip(ctx, app, app.augmentContract(*(as.Contract())), as)
				},
			})
		} else {
			items = append(items, prompt.MenuItem{
				Label: "Move ship",
				Fn: func() error {
					return moveShip(ctx, app, as)
				},
			})
		}
		items = append(items, prompt.MenuItem{
			Label: "View raw",
			Fn: func() error {
				bs, err := json.Marshal(as.Ship())
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				if err := json.Indent(&buf, bs, "", "  "); err != nil {
					return err
				}
				fmt.Println(buf.String())
				return nil
			},
		})
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
