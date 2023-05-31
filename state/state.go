package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"

	"github.com/adrg/xdg"

	"fivebit.co.uk/spacetraders/api"
	"fivebit.co.uk/spacetraders/prompt"
)

var (
	availableFactions = []string{
		"COSMIC",
		"VOID",
		"GALACTIC",
		"QUANTUM",
		"DOMINION",
	}
)

type state struct {
	filePath string
	Symbol string
	Faction string
	Email string
	Token string
	ShipAssignments map[string]string
	ContractAssignments map[string][]string
}

type State interface {
	GetToken() string
	Update(func(ms MutableState) error) error
	AssignedContract(shipID string) string
	AssignedShips(contractID string) []string
	ActiveContracts() []string
}

type MutableState interface {
	State
	AssignShip(contractID, shipID string)
	UnassignShip(contractID, shipID string)
	CompleteContract(contractID string)
}

func (s *state) GetToken() string {
	return s.Token
}

func (s *state) Update(fn func(ms MutableState) error) error {
	if err := fn(s); err != nil {
		return err
	}
	return s.write()
}

func (s *state) write() error {
	bs, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, bs, 0600)
}

func (s *state) AssignedContract(shipID string) string {
	return s.ShipAssignments[shipID]
}

func (s *state) AssignedShips(contractID string) []string {
	return s.ContractAssignments[contractID]
}

func (s *state) ActiveContracts() []string {
	var ids []string
	for cID := range s.ContractAssignments {
		ids = append(ids, cID)
	}
	return ids
}

func (s *state) AssignShip(contractID, shipID string) {
	s.ShipAssignments[shipID] = contractID
	s.ContractAssignments[contractID] = append(s.ContractAssignments[contractID], shipID)
}

func (s *state) UnassignShip(contractID, shipID string) {
	delete(s.ShipAssignments, shipID)
	var filteredShips []string
	for _, ship := range s.ContractAssignments[contractID] {
		if ship != shipID {
			filteredShips = append(filteredShips, ship)
		}
	}
	s.ContractAssignments[contractID] = filteredShips
}

func (s *state) CompleteContract(contractID string) {
	for _, shipID := range s.ContractAssignments[contractID] {
		delete(s.ShipAssignments, shipID)
	}
	delete(s.ContractAssignments, contractID)
}

func Get(ctx context.Context, client *api.APIClient) (State, error) {
	stateFilePath, err := xdg.ConfigFile(filepath.Join("spacetraders", "state.json"))
	if err != nil {
		return nil, err
	}

	bs, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return create(ctx, client, stateFilePath)
		}
		return nil, err
	}

	s := &state{}
	if err := json.Unmarshal(bs, s); err != nil {
		return nil, err
	}

	// TODO: validate state
	s.filePath = stateFilePath
	if s.ShipAssignments == nil {
		s.ShipAssignments = map[string]string{}
	}
	if s.ContractAssignments == nil {
		s.ContractAssignments = map[string][]string{}
	}

	return s, nil
}

var (
	agentSymbolRegexp = regexp.MustCompile(`[A-Z0-9]{3,14}`)
)

func create(ctx context.Context, client *api.APIClient, stateFilePath string) (State, error) {
	fmt.Println("No state found; creating new agent")

	symbol, err := prompt.Prompt("Agent symbol", func(input string) error {
		if !agentSymbolRegexp.MatchString(input) {
			return errors.New("Agent symbol must be between 3 and 14 uppercase characters or digits")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	email, err := prompt.Prompt("Email", func(input string) error {
		_, err := mail.ParseAddress(input)
		return err
	})
	if err != nil {
		return nil, err
	}

	faction, err := prompt.Select("Faction", availableFactions)
	if err != nil {
		return nil, err
	}

	resp, _, err := client.DefaultApi.Register(ctx).RegisterRequest(api.RegisterRequest{
		Symbol: symbol,
		Faction: faction,
		Email: &email,
	}).Execute()
	if err != nil {
		return nil, err
	}

	s := &state{
		filePath: stateFilePath,
		Symbol: symbol,
		Faction: faction,
		Email: email,
		Token: resp.Data.Token,
		ShipAssignments: map[string]string{},
		ContractAssignments: map[string][]string{},
	}

	fmt.Printf("Registered %s with token %s\n", symbol, s.Token)

	return s, s.write()
}
