package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"hyprtime/internal/db"
	"hyprtime/internal/logger"
	"hyprtime/internal/tracker"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	silent := flag.Bool("quiet", false, "Silent all non-error logs")
	flag.Parse()

	logger.Init()

	if *debug {
		logger.SetLevel(logger.DebugL)
	} else if *silent {
		logger.SetLevel(logger.SilentL)
	}

	logger.Info("Starting Daemon")

	db, err := db.New()
	if err != nil {
		logger.Fatal("DB error: %v", err)
	}
	defer db.Close()

	t := tracker.New(db)
	if err := t.Start(); err != nil {
		logger.Fatal("Tracker error: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down")

	t.Stop()

	logger.Info("Daemon stopped")
}
