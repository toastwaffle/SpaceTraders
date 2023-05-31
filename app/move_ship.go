package app

import (
	"context"
	"text/template"

	"fivebit.co.uk/spacetraders/prompt"
	"github.com/manifoldco/promptui"
)

var waypointShortTemplate = template.Must(template.New("waypoint_full").Parse(`{{.Symbol}}[{{.Type}}{{if .Faction}}, {{.Faction.Symbol}}{{end}}]
{{- if .Traits}} ({{range $i, $t := .Traits}}{{if gt $i 0}}, {{end}}{{$t.Name}}{{end}}){{end}}`))

func moveShip(ctx context.Context, app *App, ship *AugmentedShip) error {
	return prompt.Menu("Move ship", []prompt.MenuItem{
		{
			Label: "Input waypoint",
			Fn: func() error {
				return moveShipWaypointInput(ctx, app, ship)
			},
		},
		{
			Label: "Select waypoint in current system",
			Fn: func() error {
				return moveShipSelectWaypoint(ctx, app, ship)
			},
		},
	})
}

func moveShipWaypointInput(ctx context.Context, app *App, as *AugmentedShip) error {
	wp, err := ReadWaypoint()
	if err != nil {
		return err
	}
	_, err = as.TravelTo(ctx, wp.String())
	return err
}

func moveShipSelectWaypoint(ctx context.Context, app *App, as *AugmentedShip) error {
	waypoints, err := app.getWaypoints(ctx, as.Ship().Nav.SystemSymbol)
	if err != nil {
		return err
	}
	var items []string
	for _, wp := range waypoints {
		label, err := stringTemplate(waypointShortTemplate, wp)
		if err != nil {
			return err
		}
		items = append(items, label)
	}
	i, _, err := (&promptui.Select{
		Label: "Select waypoint",
		Items: items,
	}).Run()
	if err != nil {
		return err
	}
	_, err = as.TravelTo(ctx, waypoints[i].Symbol)
	return err
}
