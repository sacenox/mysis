package features

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/constants"
)

const (
	// maxConsecutiveErrors is the threshold for circuit breaker.
	// After this many consecutive errors, autoplay will stop.
	maxConsecutiveErrors = 3
)

// AutoplayStatus represents the current state of autoplay.
type AutoplayStatus struct {
	Enabled  bool
	Message  string
	Interval time.Duration
}

// AutoplayCallbacks defines the callback functions for autoplay events.
// These allow display-specific implementations to handle events differently.
type AutoplayCallbacks struct {
	// OnStarted is called when autoplay starts.
	OnStarted func(message string, interval time.Duration)

	// OnStopped is called when autoplay stops.
	OnStopped func()

	// OnTurn is called before sending each autoplay message.
	// Should return an error if the turn should not be processed.
	OnTurn func(ctx context.Context, message string) error

	// OnError is called when an error occurs during autoplay.
	OnError func(err error)
}

// Service manages autoplay functionality in a display-agnostic way.
// It handles the timing, state management, and loop control for autoplay,
// while delegating display-specific concerns to callbacks.
type Service struct {
	enabled           bool
	message           string
	interval          time.Duration
	cancel            context.CancelFunc
	mu                sync.Mutex
	callbacks         AutoplayCallbacks
	consecutiveErrors int // P3: Track consecutive failures for circuit breaker
}

// NewAutoplayService creates a new autoplay service with the given callbacks.
func NewAutoplayService(callbacks AutoplayCallbacks) *Service {
	return &Service{
		interval:  constants.AutoplayInterval,
		callbacks: callbacks,
	}
}

// Start begins autoplay with the given message.
// Returns an error if autoplay is already running or if inputs are invalid.
func (s *Service) Start(ctx context.Context, message string) error {
	// P2: Validate inputs
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	s.mu.Lock()
	if s.enabled {
		s.mu.Unlock()
		return fmt.Errorf("autoplay already running")
	}

	s.enabled = true
	s.message = message
	s.consecutiveErrors = 0 // P3: Reset error counter on start

	// P1: Use Background context for autoplay loop independence
	// The autoplay loop needs to run independently of the caller's context.
	// If we used the passed ctx and it gets canceled, the autoplay loop would stop unexpectedly.
	// Instead, we create our own context from Background that we control via Stop().
	autoplayCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	// Notify via callback
	if s.callbacks.OnStarted != nil {
		s.callbacks.OnStarted(message, s.interval)
	}

	log.Info().
		Str("message", message).
		Dur("interval", s.interval).
		Msg("Autoplay started")

	// Start autoplay loop in background
	go s.runLoop(autoplayCtx)

	return nil
}

// Stop stops the autoplay loop.
// Returns an error if autoplay is not running.
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return fmt.Errorf("autoplay not active")
	}

	s.enabled = false
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()

	log.Info().Msg("Autoplay stopped")
	return nil
}

// Status returns the current autoplay status.
func (s *Service) Status() AutoplayStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return AutoplayStatus{
		Enabled:  s.enabled,
		Message:  s.message,
		Interval: s.interval,
	}
}

// runLoop is the main autoplay loop that runs in a background goroutine.
func (s *Service) runLoop(ctx context.Context) {
	log.Debug().Msg("Autoplay goroutine started")

	defer func() {
		// Normal cleanup
		s.mu.Lock()
		s.enabled = false
		s.cancel = nil
		s.mu.Unlock()

		// Notify via callback
		if s.callbacks.OnStopped != nil {
			s.callbacks.OnStopped()
		}

		log.Debug().Msg("Autoplay goroutine exiting")
	}()

	// Send first message immediately
	log.Debug().Msg("Sending first autoplay message")
	if err := s.sendMessage(ctx); err != nil {
		log.Error().Err(err).Msg("Autoplay failed to send first message")
		s.mu.Lock()
		s.consecutiveErrors++
		consecutiveErrors := s.consecutiveErrors
		s.mu.Unlock()

		if s.callbacks.OnError != nil {
			s.callbacks.OnError(err)
		}

		// P3: Circuit breaker - stop if too many consecutive errors
		if consecutiveErrors >= maxConsecutiveErrors {
			log.Warn().Int("consecutive_errors", consecutiveErrors).Msg("Circuit breaker triggered - stopping autoplay")
			return
		}
		return
	}
	// Reset error counter on success
	s.mu.Lock()
	s.consecutiveErrors = 0
	s.mu.Unlock()
	log.Debug().Msg("First autoplay message sent successfully")

	// Check if canceled during first message processing
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Then wait and send subsequent messages
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			enabled := s.enabled
			s.mu.Unlock()

			if !enabled {
				return
			}

			if err := s.sendMessage(ctx); err != nil {
				log.Warn().Err(err).Msg("Autoplay turn failed")
				s.mu.Lock()
				s.consecutiveErrors++
				consecutiveErrors := s.consecutiveErrors
				s.mu.Unlock()

				if s.callbacks.OnError != nil {
					s.callbacks.OnError(err)
				}

				// P3: Circuit breaker - stop if too many consecutive errors
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Warn().Int("consecutive_errors", consecutiveErrors).Msg("Circuit breaker triggered - stopping autoplay")
					return
				}
			} else {
				// Reset error counter on success
				s.mu.Lock()
				s.consecutiveErrors = 0
				s.mu.Unlock()
			}

			// Check if canceled immediately after processing turn
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// sendMessage sends a single autoplay message by calling the OnTurn callback.
func (s *Service) sendMessage(ctx context.Context) error {
	log.Debug().Msg("sendAutoplayMessage called")

	s.mu.Lock()
	enabled := s.enabled
	message := s.message
	s.mu.Unlock()

	log.Debug().Bool("enabled", enabled).Str("message", message).Msg("Autoplay state")

	if !enabled {
		return fmt.Errorf("autoplay disabled")
	}

	// Call the OnTurn callback to process the turn
	if s.callbacks.OnTurn == nil {
		return fmt.Errorf("no OnTurn callback configured")
	}

	if err := s.callbacks.OnTurn(ctx, message); err != nil {
		return fmt.Errorf("autoplay turn failed: %w", err)
	}

	log.Debug().Msg("Autoplay message sent successfully")
	return nil
}
