package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/app"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	cfg := app.Config{
		AppName: "Cooking App",
		Message: "ready to serve",
	}

	err := app.Run(ctx, os.Stdout, cfg)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
}
