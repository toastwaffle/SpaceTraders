package app

import (
	"context"
	"text/template"
)

var agentTemplate = template.Must(template.New("agent").Parse(`
Account ID: {{.AccountId}}
Symbol: {{.Symbol}}
Headquarters: {{.Headquarters}}
Credits: {{.Credits}}
Starting Faction: {{.StartingFaction}}
`))

func viewAgent(ctx context.Context, app *App) error {
	return printTemplate(agentTemplate, app.agent)
}
