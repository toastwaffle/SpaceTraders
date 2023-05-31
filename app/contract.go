package app

import (
	"context"
	"text/template"

	"fivebit.co.uk/spacetraders/api"
	"fivebit.co.uk/spacetraders/state"
)

var contractFulfilledTemplate = template.Must(template.New("contract_fulfilled").Parse(`
{{- .Contract.Type}}[{{.Contract.FactionSymbol}}]
{{- if .Contract.Terms.Deliver}}({{range $i, $d := .Contract.Terms.Deliver}}{{if gt $i 0}}, {{end}}{{$d.TradeSymbol}}{{end}}){{end}} fulfilled{{range .Ships}}
  {{.Ship.Registration.Name}} ({{.Ship.Registration.Role}}) now unassigned{{end}}
`))

type AugmentedContract struct {
	Contract api.Contract
	Ships    []api.Ship
}

func (ac *AugmentedContract) Active() bool {
	return len(ac.Ships) > 0
}

func (a *App) augmentContract(c api.Contract) *AugmentedContract {
	ac := &AugmentedContract{Contract: c}
	for _, shipID := range a.state.AssignedShips(c.Id) {
		ac.Ships = append(ac.Ships, a.ships[shipID])
	}
	return ac
}

func (a *App) fulfillContract(ctx context.Context, cID string) error {
	resp, _, err := a.client.ContractsApi.FulfillContract(ctx, cID).Execute()
	if err != nil {
		return err
	}
	a.agent = resp.Data.Agent
	printTemplate(contractFulfilledTemplate, a.augmentContract(a.activeContracts[cID]))
	delete(a.activeContracts, cID)
	return a.state.Update(func(ms state.MutableState) error {
		ms.CompleteContract(cID)
		return nil
	})
}
