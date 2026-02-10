package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/styles"
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
}

// ParseFlags parses command-line flags and returns the result.
func ParseFlags(version string) *Flags {
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

	flag.Usage = func() {
		printHelp(version)
	}

	flag.Parse()

	// Handle version flag
	if f.ShowVersion {
		fmt.Printf("Mysis %s\n", version)
		os.Exit(0)
	}

	// Handle help flag
	if f.ShowHelp {
		printHelp(version)
		os.Exit(0)
	}

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

// printHelp displays usage information.
func printHelp(version string) {
	fmt.Println(styles.Brand.Render("╔══════════════════════════════════════╗"))
	fmt.Println(styles.Brand.Render("║") + "  " + styles.BrandBold.Render("Mysis") + " - SpaceMolt Agent CLI         " + styles.Brand.Render("║"))
	fmt.Println(styles.Brand.Render("╚══════════════════════════════════════╝"))
	fmt.Println()
	fmt.Println(styles.BrandBold.Render("USAGE:"))
	fmt.Println("  mysis [flags]")
	fmt.Println()
	fmt.Println(styles.BrandBold.Render("FLAGS:"))
	fmt.Println("  " + styles.Secondary.Render("-h, --help") + "              Show this help message")
	fmt.Println("  " + styles.Secondary.Render("-v, --version") + "           Show version information")
	fmt.Println("  " + styles.Secondary.Render("-c, --config") + " PATH       Path to config file (default: config.toml)")
	fmt.Println("  " + styles.Secondary.Render("-d, --debug") + "             Enable debug logging")
	fmt.Println("  " + styles.Secondary.Render("-p, --provider") + " NAME     Provider name (overrides config default)")
	fmt.Println("  " + styles.Secondary.Render("-s, --session") + " NAME      Session name (resume or create)")
	fmt.Println("  " + styles.Secondary.Render("-a, --autoplay") + " MSG      Start autoplay immediately with message")
	fmt.Println("  " + styles.Secondary.Render("-f, --file") + " PATH      Load system prompt from markdown file")
	fmt.Println("  " + styles.Secondary.Render("-l, --list-sessions") + "     List recent sessions and exit")
	fmt.Println("  " + styles.Secondary.Render("-D, --delete-session") + " N  Delete session by name and exit")
	fmt.Println()
	fmt.Println(styles.BrandBold.Render("EXAMPLES:"))
	fmt.Println("  # Start anonymous session")
	fmt.Println("  mysis")
	fmt.Println()
	fmt.Println("  # Resume or create named session")
	fmt.Println("  mysis -s mybot")
	fmt.Println()
	fmt.Println("  # Start with autoplay enabled")
	fmt.Println("  mysis -s mybot -a \"explore and mine resources\"")
	fmt.Println()
	fmt.Println("  # List all sessions")
	fmt.Println("  mysis -l")
	fmt.Println()
	fmt.Println("  # Delete a session")
	fmt.Println("  mysis -D mybot")
	fmt.Println()
	fmt.Println(styles.BrandBold.Render("IN-SESSION COMMANDS:"))
	fmt.Println("  " + styles.Secondary.Render("/autoplay <message>") + "    Start autonomous gameplay with given goal")
	fmt.Println("  " + styles.Secondary.Render("/autoplay stop") + "         Stop autonomous gameplay")
	fmt.Println("  " + styles.Secondary.Render("exit, quit") + "             Exit the session")
	fmt.Println()
	fmt.Println(styles.Muted.Render("Note: Running without -s/--session creates an anonymous session (not saved by name)."))
	fmt.Println()
}
