package cli

import (
	"context"

	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/session"
	"github.com/xonecas/mysis/internal/tui"
)

// runTUI starts the TUI mode.
func runTUI(
	ctx context.Context,
	sessionMgr *session.Manager,
	sessionID string,
	prov provider.Provider,
	proxy *mcp.Proxy,
	tools []mcp.Tool,
	history []provider.Message,
) error {
	// Create TUI runner
	runner := tui.NewRunner(
		ctx,
		sessionMgr,
		sessionID,
		prov,
		proxy,
		tools,
		history,
	)

	// Run TUI
	return runner.Run()
}
