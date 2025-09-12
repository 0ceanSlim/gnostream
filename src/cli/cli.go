package cli

import (
	"flag"
	"fmt"
	"os"

	"gnostream/src/cli/commands"
	"gnostream/src/config"
)

// Version information set at build time
var (
	Version   = "v0.0.0-dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// CLI represents the command line interface
type CLI struct {
	config *config.Config
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{}
}

// Run executes the CLI based on command line arguments
func (cli *CLI) Run() error {
	if len(os.Args) < 2 {
		cli.printUsage()
		return nil
	}

	command := os.Args[1]

	switch command {
	case "server":
		return cli.runServer()
	case "config":
		return cli.runConfig()
	case "events":
		return cli.runEvents()
	case "stream":
		return cli.runStream()
	case "cleanup":
		return cli.runCleanup()
	case "version":
		return cli.runVersion()
	case "help", "-h", "--help":
		cli.printUsage()
		return nil
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		cli.printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

// printUsage prints the CLI usage information
func (cli *CLI) printUsage() {
	fmt.Println(`🎬 GNOSTREAM CLI

USAGE:
    gnostream <COMMAND> [OPTIONS]

COMMANDS:
    server          Start the streaming server (default mode)
    config          Manage configuration settings
    events          Manage Nostr stream events
    stream          Stream management and debugging
    cleanup         Clean up stale streams and events  
    version         Show version information
    help            Show this help message

EXAMPLES:
    gnostream server                    # Start the streaming server
    gnostream config get recording      # Get current recording setting
    gnostream config set recording true # Enable recording
    gnostream events list               # List all stream events
    gnostream events delete <id>        # Delete specific event
    gnostream stream status             # Show current stream status
    gnostream cleanup stale             # Clean up stale live events
    
For more information on a specific command, use:
    gnostream <COMMAND> --help`)
}

// loadConfig loads the configuration for CLI operations
func (cli *CLI) loadConfig() error {
	if cli.config != nil {
		return nil
	}

	cfg, err := config.Load("config.yml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cli.config = cfg
	return nil
}

// runServer starts the streaming server
func (cli *CLI) runServer() error {
	// This will be called from main.go when running server mode
	return nil
}

// runConfig handles configuration commands
func (cli *CLI) runConfig() error {
	if err := cli.loadConfig(); err != nil {
		return err
	}

	configCmd := commands.NewConfigCommand(cli.config)
	return configCmd.Execute(os.Args[2:])
}

// runEvents handles Nostr event management
func (cli *CLI) runEvents() error {
	if err := cli.loadConfig(); err != nil {
		return err
	}

	eventsCmd := commands.NewEventsCommand(cli.config)
	return eventsCmd.Execute(os.Args[2:])
}

// runStream handles stream management
func (cli *CLI) runStream() error {
	if err := cli.loadConfig(); err != nil {
		return err
	}

	streamCmd := commands.NewStreamCommand(cli.config)
	return streamCmd.Execute(os.Args[2:])
}

// runCleanup handles cleanup operations
func (cli *CLI) runCleanup() error {
	if err := cli.loadConfig(); err != nil {
		return err
	}

	cleanupCmd := commands.NewCleanupCommand(cli.config)
	return cleanupCmd.Execute(os.Args[2:])
}

// runVersion shows version information
func (cli *CLI) runVersion() error {
	fmt.Printf("gnostream %s\n", Version)
	fmt.Println("A decentralized live streaming solution")
	fmt.Printf("Built: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n", GitCommit)
	return nil
}

// ParseFlags parses common CLI flags
func ParseFlags(args []string) (*flag.FlagSet, error) {
	fs := flag.NewFlagSet("gnostream", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Use 'gnostream help' for usage information\n")
	}
	
	err := fs.Parse(args)
	return fs, err
}