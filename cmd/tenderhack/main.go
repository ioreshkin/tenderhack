package main

import (
	"context"
	"log"
	"os"

	"tenderhack/internal/app"
)

func main() {
	cfg := app.LoadConfig()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg app.Config) error {
	if len(os.Args) < 2 {
		return app.RunServe(context.Background(), cfg)
	}

	switch os.Args[1] {
	case "serve":
		return app.RunServe(context.Background(), cfg)
	case "import":
		return app.RunImport(context.Background(), cfg)
	case "init-db":
		return app.RunInitDB(context.Background(), cfg)
	default:
		return app.ErrUnknownCommand(os.Args[1])
	}
}
