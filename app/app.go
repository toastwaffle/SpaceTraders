package app

import (
	"context"
	"time"

	"fivebit.co.uk/spacetraders/api"
	"fivebit.co.uk/spacetraders/prompt"
	"fivebit.co.uk/spacetraders/state"
)

type App struct {
	state           state.State
	client          *api.APIClient
	agent           api.Agent
	ships           map[string]api.Ship
	activeContracts map[string]api.Contract
	waypoints       map[string][]api.Waypoint
	surveys         map[string]map[string]*api.Survey
	shipReadyTimes   map[string]time.Time
}

func (a *App) MenuItem(ctx context.Context, label string, fn func(ctx context.Context, app *App) error) prompt.MenuItem {
	return prompt.MenuItem{
		Label: label,
		Fn: func() error {
			return fn(ctx, a)
		},
		Loop: true,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.loadData(ctx); err != nil {
		return err
	}
	return prompt.Menu("Choose action", []prompt.MenuItem{
		a.MenuItem(ctx, "Run activity", runActivityLoop),
		a.MenuItem(ctx, "View surveys", viewSurveys),
		a.MenuItem(ctx, "View agent", viewAgent),
		a.MenuItem(ctx, "View waypoint", viewWaypoint),
		a.MenuItem(ctx, "View contracts", viewContracts),
		a.MenuItem(ctx, "View fleet", viewFleet),
		a.MenuItem(ctx, "Buy ship", buyShip),
		{
			Label: "Reload data",
			Fn: func() error {
				return a.loadData(ctx)
			},
			Loop: true,
		},
		prompt.MenuItemQuit,
	})
}

func Run(ctx context.Context) error {
	client := api.NewAPIClient(api.NewConfiguration())
	s, err := state.Get(ctx, client)
	if err != nil {
		return err
	}
	app := &App{
		state:  s,
		client: client,
	}
	ctx = context.WithValue(ctx, api.ContextAccessToken, s.GetToken())
	return app.Run(ctx)
}
