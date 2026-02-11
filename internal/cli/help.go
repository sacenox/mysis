package cli

import (
	"fmt"

	"github.com/xonecas/mysis/internal/styles"
)

// PrintVersion displays the version information.
func PrintVersion(version string) {
	fmt.Printf("Mysis %s\n", version)
}

// PrintHelp displays usage information with CLI styling.
func PrintHelp(version string) {
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
	fmt.Println("  " + styles.Secondary.Render("-t, --tui") + "              Use terminal UI mode")
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
