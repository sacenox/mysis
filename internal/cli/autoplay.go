package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/constants"
	"github.com/xonecas/mysis/internal/features"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/styles"
)

// initAutoplayService initializes the autoplay service with CLI-specific callbacks.
// This should be called once when creating the App.
func (app *App) initAutoplayService() {
	app.autoplayService = features.NewAutoplayService(features.AutoplayCallbacks{
		OnStarted: func(message string, interval time.Duration) {
			fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay started: \"%s\"", message)))
			fmt.Println(styles.Muted.Render(fmt.Sprintf("Interval: %ds (%d avg tool calls × %ds/tick)",
				int(interval.Seconds()),
				constants.AvgToolCallsPerTurn,
				int(constants.GameTickDuration.Seconds()))))
			fmt.Println(styles.Muted.Render("Type '/autoplay stop' to stop"))
			fmt.Println()
		},
		OnStopped: func() {
			fmt.Println(styles.Muted.Render("Autoplay stopped"))
		},
		OnTurn: func(ctx context.Context, message string) error {
			fmt.Println(styles.Muted.Render("─── Autoplay Turn ───"))
			fmt.Println(styles.Brand.Render("> ") + message)
			log.Debug().Msg("About to process turn")

			// Send autoplay message
			userMsg := provider.Message{
				Role:    "user",
				Content: message,
			}

			app.mu.Lock()
			app.history = append(app.history, userMsg)
			app.mu.Unlock()

			if err := app.sessionMgr.SaveMessage(app.sessionID, userMsg); err != nil {
				log.Warn().Err(err).Msg("Failed to save autoplay message")
			}

			// Process turn
			if err := app.processTurn(ctx); err != nil {
				fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
				// Don't stop autoplay on errors - just log and continue
				log.Warn().Err(err).Msg("Autoplay turn failed, continuing...")
			}

			fmt.Println() // Blank line after response
			return nil
		},
		OnError: func(err error) {
			log.Error().Err(err).Msg("Autoplay error")
		},
	})
}

// startAutoplayFromFlag starts autoplay from CLI flag.
func (app *App) startAutoplayFromFlag(ctx context.Context, message string) error {
	return app.autoplayService.Start(ctx, message)
}

// handleAutoplayCommand handles /autoplay commands
func (app *App) handleAutoplayCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)

	if len(parts) == 1 {
		// Just "/autoplay" - show status
		status := app.autoplayService.Status()
		if status.Enabled {
			fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay active: \"%s\"", status.Message)))
		} else {
			fmt.Println(styles.Muted.Render("Autoplay not active"))
			fmt.Println(styles.Muted.Render("Usage: /autoplay <message>"))
		}
		return nil
	}

	// Check for "stop" subcommand
	if parts[1] == "stop" {
		if err := app.autoplayService.Stop(); err != nil {
			fmt.Println(styles.Muted.Render(err.Error()))
		} else {
			fmt.Println(styles.Success.Render("Autoplay stopped"))
		}
		return nil
	}

	// Join all parts after /autoplay as the message
	message := strings.Join(parts[1:], " ")

	if message == "" {
		return fmt.Errorf("missing message for autoplay")
	}

	// Start autoplay
	if err := app.autoplayService.Start(ctx, message); err != nil {
		return fmt.Errorf("%s - use '/autoplay stop' first", err.Error())
	}

	return nil
}
