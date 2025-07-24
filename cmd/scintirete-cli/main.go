// Package main provides the command-line interface for Scintirete.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scintirete/scintirete/cmd/scintirete-cli/cli"
)

var (
	version = "dev"
	commit  = "unknown"

	// Command line flags
	host     = flag.String("h", "localhost", "Server host")
	port     = flag.Int("p", 9090, "Server port")
	password = flag.String("a", "", "Password for authentication")
	database = flag.String("d", "", "Database name")
	help     = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Parse()

	if *help {
		showUsage()
		return
	}

	// Set version information
	cli.SetVersion(version, commit)

	// Create CLI instance
	cliInstance := cli.NewCLI(*password)

	// Connect to server
	if err := cliInstance.Connect(*host, *port); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to server: %v\n", err)
		os.Exit(1)
	}
	defer cliInstance.Close()

	// Set database if specified
	if *database != "" {
		cli.SetCurrentDatabase(*database)
		cliInstance.SetPrompt(fmt.Sprintf("scintirete[%s]> ", *database))
	}

	// Check if there are command line arguments to execute
	args := flag.Args()
	if len(args) > 0 {
		// Execute single command and exit
		if err := cliInstance.ExecuteCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start interactive mode
	cliInstance.Interactive(version, commit)
}

// showUsage displays the usage information
func showUsage() {
	fmt.Printf("Scintirete CLI %s - Command-line interface for Scintirete vector database\n\n", version)
	fmt.Println("Usage:")
	fmt.Printf("  %s [options] [command]\n\n", os.Args[0])
	fmt.Println("Options:")
	fmt.Println("  -h <host>        Server host (default: localhost)")
	fmt.Println("  -p <port>        Server port (default: 9090)")
	fmt.Println("  -a <password>    Authentication password")
	fmt.Println("  -d <database>    Default database to use")
	fmt.Println("  --help           Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s -h localhost -p 9090 -a mypassword\n", os.Args[0])
	fmt.Printf("  %s ping\n", os.Args[0])
	fmt.Printf("  %s -d mydb list-collections\n", os.Args[0])
	fmt.Println()
	fmt.Println("Interactive mode:")
	fmt.Println("  Run without arguments to enter interactive mode.")
	fmt.Println("  Type 'help' for available commands.")
}
