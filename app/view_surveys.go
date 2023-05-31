package app

import (
	"context"
	"fmt"
	"strings"

	"fivebit.co.uk/spacetraders/api"
)

func formatSurvey(survey api.Survey) string {
	var pieces []string
	mineralCount, _ := mineralCounts(survey)
	for symbol, count := range mineralCount {
		pieces = append(pieces, fmt.Sprintf("%sx%d", symbol, count))
	}
	return strings.Join(pieces, ", ")
}

func viewSurveys(ctx context.Context, app *App) error {
	fmt.Println()
	for waypoint, surveys := range app.surveys {
		fmt.Println(waypoint)
		for mineral, survey := range surveys {
			fmt.Printf("  %s: %s\n", mineral, formatSurvey(*survey))
		}
	}
	fmt.Println()
	return nil
}
