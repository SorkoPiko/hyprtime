package main

import (
	"log"

	"hyprtime/internal/db"
	"hyprtime/internal/ui"
)

func main() {
	db, err := db.New()
	if err != nil {
		log.Fatalf("DB init err: %v\n", err)
	}
	defer db.Close()

	app := ui.New(db)
	if err := app.Run(); err != nil {
		log.Fatalf("App run err: %v\n", err)
	}
}
