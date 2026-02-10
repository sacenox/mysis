package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/constants"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/styles"
)

// startAutoplayFromFlag starts autoplay from CLI flag.
func (app *App) startAutoplayFromFlag(ctx context.Context, message string) error {
	app.mu.Lock()
	if app.autoplayEnabled {
		app.mu.Unlock()
		return fmt.Errorf("autoplay already running")
	}

	app.autoplayEnabled = true
	app.autoplayMessage = message

	// Create cancelable context for autoplay goroutine
	autoplayCtx, cancel := context.WithCancel(ctx)
	app.autoplayCancel = cancel
	app.mu.Unlock()

	fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay started: \"%s\"", message)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Interval: %ds (%d avg tool calls × %ds/tick)",
		int(constants.AutoplayInterval.Seconds()),
		constants.AvgToolCallsPerTurn,
		int(constants.GameTickDuration.Seconds()))))
	fmt.Println(styles.Muted.Render("Type '/autoplay stop' to stop"))
	fmt.Println()

	// Start autoplay loop in background
	go app.runAutoplay(autoplayCtx)

	return nil
}

// handleAutoplayCommand handles /autoplay commands
func (app *App) handleAutoplayCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)

	if len(parts) == 1 {
		// Just "/autoplay" - show status
		app.mu.Lock()
		enabled := app.autoplayEnabled
		message := app.autoplayMessage
		app.mu.Unlock()

		if enabled {
			fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay active: \"%s\"", message)))
		} else {
			fmt.Println(styles.Muted.Render("Autoplay not active"))
			fmt.Println(styles.Muted.Render("Usage: /autoplay <message>"))
		}
		return nil
	}

	// Check for "stop" subcommand
	if parts[1] == "stop" {
		app.mu.Lock()
		if app.autoplayEnabled {
			app.autoplayEnabled = false
			if app.autoplayCancel != nil {
				app.autoplayCancel()
				app.autoplayCancel = nil
			}
			app.mu.Unlock()
			fmt.Println(styles.Success.Render("Autoplay stopped"))
		} else {
			app.mu.Unlock()
			fmt.Println(styles.Muted.Render("Autoplay not active"))
		}
		return nil
	}

	// Join all parts after /autoplay as the message
	message := strings.Join(parts[1:], " ")

	if message == "" {
		return fmt.Errorf("missing message for autoplay")
	}

	// Start autoplay
	app.mu.Lock()
	if app.autoplayEnabled {
		app.mu.Unlock()
		return fmt.Errorf("autoplay already running - use '/autoplay stop' first")
	}

	app.autoplayEnabled = true
	app.autoplayMessage = message

	// Create cancelable context for autoplay goroutine
	autoplayCtx, cancel := context.WithCancel(ctx)
	app.autoplayCancel = cancel
	app.mu.Unlock()

	fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay started: \"%s\"", message)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Interval: %ds (%d avg tool calls × %ds/tick)",
		int(constants.AutoplayInterval.Seconds()),
		constants.AvgToolCallsPerTurn,
		int(constants.GameTickDuration.Seconds()))))
	fmt.Println(styles.Muted.Render("Type '/autoplay stop' to stop"))

	// Start autoplay loop in background
	go app.runAutoplay(autoplayCtx)

	return nil
}

// runAutoplay sends the autoplay message at intervals based on expected tool call duration.
func (app *App) runAutoplay(ctx context.Context) {
	log.Debug().Msg("Autoplay goroutine started")

	defer func() {
		app.mu.Lock()
		app.autoplayEnabled = false
		app.autoplayCancel = nil
		app.mu.Unlock()
		fmt.Println(styles.Muted.Render("Autoplay stopped"))
		log.Debug().Msg("Autoplay goroutine exiting")
	}()

	// Send first message immediately
	log.Debug().Msg("Sending first autoplay message")
	if err := app.sendAutoplayMessage(ctx); err != nil {
		log.Error().Err(err).Msg("Autoplay failed to send first message")
		return
	}
	log.Debug().Msg("First autoplay message sent successfully")

	// Then wait and send subsequent messages
	ticker := time.NewTicker(constants.AutoplayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.mu.Lock()
			enabled := app.autoplayEnabled
			app.mu.Unlock()

			if !enabled {
				return
			}

			if err := app.sendAutoplayMessage(ctx); err != nil {
				return
			}
		}
	}
}

// sendAutoplayMessage sends the autoplay message and processes the turn
func (app *App) sendAutoplayMessage(ctx context.Context) error {
	log.Debug().Msg("sendAutoplayMessage called")

	app.mu.Lock()
	enabled := app.autoplayEnabled
	message := app.autoplayMessage
	app.mu.Unlock()

	log.Debug().Bool("enabled", enabled).Str("message", message).Msg("Autoplay state")

	if !enabled {
		return fmt.Errorf("autoplay disabled")
	}

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
	log.Debug().Msg("Autoplay message sent successfully")
	return nil
}
