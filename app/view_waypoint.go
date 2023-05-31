package app

import (
	"context"
	"text/template"
)

var waypointFullTemplate = template.Must(template.New("waypoint_full").Parse(`
Symbol: {{.Symbol}}
Type: {{.Type}}{{if .Faction}}
Faction: {{.Faction.Symbol}}{{end}}{{if .Orbitals}}
Orbitals:{{range .Orbitals}}
  {{.Symbol}}{{end}}{{end}}{{if .Traits}}
Traits:{{range .Traits}}
  {{.Name}} - {{.Description}}{{end}}{{end}}
`))

func viewWaypoint(ctx context.Context, app *App) error {
	wp, err := ReadWaypoint()
	if err != nil {
		return err
	}

	resp, _, err := app.client.SystemsApi.GetWaypoint(ctx, wp.system.String(), wp.String()).Execute()
	if err != nil {
		return err
	}

	return printTemplate(waypointFullTemplate, resp.Data)
}
