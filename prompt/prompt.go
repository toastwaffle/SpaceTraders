package prompt

import (
	"errors"
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
)

func Prompt(label string, validate func(input string) error) (string, error) {
	return (&promptui.Prompt{
		Label: label,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }} ",
			Valid:   "{{ . | green }} ",
			Invalid: "{{ . | red }} ",
			Success: "{{ . | bold }} ",
		},
		Validate: validate,
	}).Run()
}

func NoValidate(_ string) error {
	return nil
}

func Select(label string, items []string) (string, error) {
	_, val, err := (&promptui.Select{
		Label: label,
		Items: items,
	}).Run()
	return val, err
}

type MenuItem struct {
	Label string
	Fn    func() error
	Loop bool
}

func (mi MenuItem) String() string {
	return mi.Label
}

func Menu(label string, items []MenuItem) error {
	loop := true
	for loop {
		i, _, err := (&promptui.Select{
			Label: label,
			Items: items,
		}).Run()
		if err != nil {
			return err
		}
		if err := items[i].Fn(); err != nil {
			return err
		}
		loop = items[i].Loop
	}
	return nil
}

var MenuItemQuit = MenuItem{
	Label: "Quit",
	Fn: func() error {
		fmt.Println("TTFN!")
		os.Exit(0)
		return nil
	},
}

var MenuItemBack = MenuItem{
	Label: "Back",
	Fn: func() error {
		return nil
	},
}

var ErrGoBack = errors.New("go back")

var MenuItemBackErr = MenuItem{
	Label: "Back",
	Fn: func() error {
		return ErrGoBack
	},
}

type MenuItemWithResult[T any] struct {
	Label string
	Fn func() (T, error)
}

func (mi MenuItemWithResult[T]) String() string {
	return mi.Label
}

func MenuWithResult[T any](label string, items []MenuItemWithResult[T]) (T, error) {
	i, _, err := (&promptui.Select{
		Label: label,
		Items: items,
	}).Run()
	if err != nil {
		var empty T
		return empty, err
	}
	return items[i].Fn()
}
