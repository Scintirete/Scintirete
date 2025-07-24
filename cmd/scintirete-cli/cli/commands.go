package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

var (
	appVersion string
	appCommit  string
)

// SetVersion sets the application version and commit
func SetVersion(version, commit string) {
	appVersion = version
	appCommit = commit
}

// GetCommands returns the commands registry
func GetCommands() map[string]Command {
	return map[string]Command{
		"help":       {Name: "help", Description: "Show help information", Usage: "help [command]", Handler: (*CLI).helpCommand},
		"quit":       {Name: "quit", Description: "Exit the CLI", Usage: "quit", Handler: (*CLI).quitCommand},
		"exit":       {Name: "exit", Description: "Exit the CLI", Usage: "exit", Handler: (*CLI).quitCommand},
		"ping":       {Name: "ping", Description: "Test connection to server", Usage: "ping", Handler: (*CLI).pingCommand},
		"version":    {Name: "version", Description: "Show version information", Usage: "version", Handler: (*CLI).versionCommand},
		"use":        {Name: "use", Description: "Switch to a database", Usage: "use <database>", Handler: (*CLI).useCommand},
		"database":   {Name: "database", Description: "Database operations", Usage: "database <list|create|drop> [args...]", Handler: (*CLI).databaseCommand},
		"collection": {Name: "collection", Description: "Collection operations", Usage: "collection <list|create|drop|info> [args...]", Handler: (*CLI).collectionCommand},
		"vector":     {Name: "vector", Description: "Vector operations", Usage: "vector <insert|search|delete> [args...]", Handler: (*CLI).vectorCommand},
		"text":       {Name: "text", Description: "Text embedding operations", Usage: "text <insert|search|models> <args...>", Handler: (*CLI).textCommand},
		"save":       {Name: "save", Description: "Synchronously save RDB snapshot", Usage: "save", Handler: (*CLI).saveCommand},
		"bgsave":     {Name: "bgsave", Description: "Asynchronously save RDB snapshot", Usage: "bgsave", Handler: (*CLI).bgsaveCommand},
	}
}

// helpCommand shows help information
func (c *CLI) helpCommand(args []string) error {
	commands := GetCommands()
	if len(args) == 0 {
		fmt.Println("Available commands:")
		fmt.Println()
		for _, cmd := range commands {
			fmt.Printf("  %-20s %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println()
		fmt.Println("Sub-commands:")
		fmt.Println("  database list              List all databases")
		fmt.Println("  database create <name>     Create a new database")
		fmt.Println("  database drop <name>       Drop a database")
		fmt.Println()
		fmt.Println("  collection list            List collections in current database")
		fmt.Println("  collection create <name> <metric> [m] [ef_construction]  Create a collection")
		fmt.Println("  collection drop <name>     Drop a collection")
		fmt.Println("  collection info <name>     Get collection information")
		fmt.Println()
		fmt.Println("  vector insert <collection> <vector> [metadata]          Insert vectors (ID auto-generated)")
		fmt.Println("  vector search <collection> <vector> [top-k] [ef-search] Search vectors")
		fmt.Println("  vector delete <collection> <id1> [id2] ...              Delete vectors")
		fmt.Println()
		fmt.Println("  text insert <collection> [model] <text> [metadata]      Insert text with embedding (ID auto-generated)")
		fmt.Println("  text search <collection> [model] <text> [top-k] [ef-search] Search text with embedding")
		fmt.Println("  text models                                               List available embedding models")
		fmt.Println()
		fmt.Println("Type 'help <command>' for detailed usage information.")
	} else {
		cmdName := strings.ToLower(args[0])
		if cmd, exists := commands[cmdName]; exists {
			fmt.Printf("%s - %s\n", cmd.Name, cmd.Description)
			fmt.Printf("Usage: %s\n", cmd.Usage)

			// Provide detailed help for sub-commands
			switch cmdName {
			case "database":
				fmt.Println("\nSub-commands:")
				fmt.Println("  list               List all databases")
				fmt.Println("  create <name>      Create a new database")
				fmt.Println("  drop <name>        Drop a database")
			case "collection":
				fmt.Println("\nSub-commands:")
				fmt.Println("  list                             List collections in current database")
				fmt.Println("  create <name> <metric> [params]  Create a collection")
				fmt.Println("    Metrics: L2, COSINE, INNER_PRODUCT")
				fmt.Println("    Optional params: <m> <ef_construction>")
				fmt.Println("  drop <name>                      Drop a collection")
				fmt.Println("  info <name>                      Get collection information")
			case "vector":
				fmt.Println("\nSub-commands:")
				fmt.Println("  insert <collection> <vector> [metadata]          Insert vectors (ID auto-generated)")
				fmt.Println("    Vector format: JSON array, e.g., [1.0, 2.0, 3.0]")
				fmt.Println("  search <collection> <vector> [top-k] [ef-search] Search vectors")
				fmt.Println("  delete <collection> <id1> [id2] ...              Delete vectors")
			case "text":
				fmt.Println("\nSub-commands:")
				fmt.Println("  insert <collection> [model] <text> [metadata]      Insert text with embedding (ID auto-generated)")
				fmt.Println("  search <collection> [model] <text> [top-k] [ef-search] Search text with embedding")
				fmt.Println("  models                                               List available embedding models")
			}
		} else {
			return fmt.Errorf("unknown command: %s", cmdName)
		}
	}
	return nil
}

// quitCommand exits the CLI
func (c *CLI) quitCommand(args []string) error {
	fmt.Println("Goodbye!")
	os.Exit(0)
	return nil
}

// pingCommand tests connection to the server
func (c *CLI) pingCommand(args []string) error {
	start := time.Now()

	// Use list databases as a simple ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.ListDatabases(ctx, &pb.ListDatabasesRequest{
		Auth: &pb.AuthInfo{Password: c.password},
	})

	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("PONG (%.2fms)\n", float64(duration.Nanoseconds())/1e6)
	return nil
}

// versionCommand shows version information
func (c *CLI) versionCommand(args []string) error {
	fmt.Printf("Scintirete CLI %s\n", appVersion)
	fmt.Printf("Commit: %s\n", appCommit)
	return nil
}

// useCommand switches to a database
func (c *CLI) useCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: use <database>")
	}

	database := args[0]
	SetCurrentDatabase(database)
	c.prompt = fmt.Sprintf("scintirete[%s]> ", database)
	fmt.Printf("Switched to database '%s'.\n", database)
	return nil
}
