package main

import (
	"flag"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
	"github.com/ayanrajpoot10/hyprtime/internal/logger"
	"github.com/ayanrajpoot10/hyprtime/internal/tracker"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug logging")
	silent := flag.Bool("silent", false, "Silent all non-error logs")
	idleTimeout := flag.Duration("idle", 2*time.Minute, "Pause tracking after this long without input (0 disables)")
	typingStats := flag.Bool("typing", true, "Count typing statistics (needs read access to /dev/input)")
	typingExclude := flag.String("typing-exclude", `^(steam_app_\d+|gamescope)$`, "Regex of window classes whose keys aren't typing")
	flag.Parse()

	logger.Init()

	exclude, err := regexp.Compile(*typingExclude)
	if err != nil {
		logger.Fatalf("Invalid -typing-exclude: %v", err)
	}

	if *debug {
		logger.SetLevel(logger.Debug)
	} else if *silent {
		logger.SetLevel(logger.Silent)
	}

	logger.Infof("Starting Daemon")

	db, err := db.New()
	if err != nil {
		logger.Fatalf("DB error: %v", err)
	}
	defer db.Close()

	t := tracker.New(db, tracker.Options{
		IdleTimeout:   *idleTimeout,
		Typing:        *typingStats,
		TypingExclude: exclude,
	})
	if err := t.Start(); err != nil {
		logger.Fatalf("Tracker error: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Infof("Shutting down")

	t.Stop()

	logger.Infof("Daemon stopped")
}
