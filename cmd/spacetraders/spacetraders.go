package main

import (
	"context"
	"log"

	"fivebit.co.uk/spacetraders/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
