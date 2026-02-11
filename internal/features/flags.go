package features

import (
	"flag"
	"os"

	"github.com/xonecas/mysis/internal/config"
)

// Flags holds parsed command-line flags.
type Flags struct {
	ShowHelp      bool
	ShowVersion   bool
	ConfigPath    string
	Debug         bool
	ProviderName  string
	SessionName   string
	ListSessions  bool
	DeleteSession string
	Autoplay      string
	SystemFile    string
	TUI           bool
}

// ParseFlags parses command-line flags and returns the result.
// This is display-agnostic - it only parses flags without printing or exiting.
// The caller is responsible for handling ShowHelp and ShowVersion flags.
func ParseFlags() *Flags {
	var f Flags

	flag.BoolVar(&f.ShowHelp, "help", false, "Show help and exit")
	flag.BoolVar(&f.ShowHelp, "h", false, "Show help and exit (shorthand)")
	flag.BoolVar(&f.ShowVersion, "version", false, "Show version and exit")
	flag.BoolVar(&f.ShowVersion, "v", false, "Show version and exit (shorthand)")
	flag.StringVar(&f.ConfigPath, "config", "", "Path to config file")
	flag.StringVar(&f.ConfigPath, "c", "", "Path to config file (shorthand)")
	flag.BoolVar(&f.Debug, "debug", false, "Enable debug logging")
	flag.BoolVar(&f.Debug, "d", false, "Enable debug logging (shorthand)")
	flag.StringVar(&f.ProviderName, "provider", "", "Provider name (overrides default from config)")
	flag.StringVar(&f.ProviderName, "p", "", "Provider name (shorthand)")
	flag.StringVar(&f.SessionName, "session", "", "Session name (resume or create named session)")
	flag.StringVar(&f.SessionName, "s", "", "Session name (shorthand)")
	flag.BoolVar(&f.ListSessions, "list-sessions", false, "List recent sessions and exit")
	flag.BoolVar(&f.ListSessions, "l", false, "List recent sessions and exit (shorthand)")
	flag.StringVar(&f.DeleteSession, "delete-session", "", "Delete a session by name")
	flag.StringVar(&f.DeleteSession, "D", "", "Delete a session by name (shorthand)")
	flag.StringVar(&f.Autoplay, "autoplay", "", "Start autoplay immediately with given message")
	flag.StringVar(&f.Autoplay, "a", "", "Start autoplay immediately (shorthand)")
	flag.StringVar(&f.SystemFile, "file", "", "Load system prompt from markdown file")
	flag.StringVar(&f.SystemFile, "f", "", "Load system prompt from markdown file (shorthand)")
	flag.BoolVar(&f.TUI, "tui", false, "Use terminal UI mode instead of CLI")
	flag.BoolVar(&f.TUI, "t", false, "Use terminal UI mode (shorthand)")

	// Disable default help behavior - caller will handle it
	flag.Usage = func() {}

	flag.Parse()

	// Resolve config path if not specified
	if f.ConfigPath == "" {
		if _, err := os.Stat("config.toml"); err == nil {
			f.ConfigPath = "config.toml"
		} else {
			dataDir, err := config.DataDir()
			if err == nil {
				f.ConfigPath = dataDir + "/config.toml"
			}
		}
	}

	return &f
}
