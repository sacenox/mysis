package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Parse flags
	var (
		showVersion = flag.Bool("version", false, "Show version and exit")
		configPath  = flag.String("config", "config.toml", "Path to config file")
		debug       = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Mysis %s\n", Version)
		os.Exit(0)
	}

	// Initialize logging
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// TODO: Initialize application
	log.Info().
		Str("version", Version).
		Str("config", *configPath).
		Msg("Starting Mysis")

	fmt.Println("Mysis - SpaceMolt Client")
	fmt.Println("TODO: Initialize application")
}
