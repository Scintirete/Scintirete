package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// databaseCommand handles database operations
func (c *CLI) databaseCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: database <list|create|drop> [args...]")
	}

	subCommand := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subCommand {
	case "list":
		return c.listDatabasesCommand(subArgs)
	case "create":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: database create <name>")
		}
		return c.createDatabaseCommand(subArgs)
	case "drop":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: database drop <name>")
		}
		return c.dropDatabaseCommand(subArgs)
	default:
		return fmt.Errorf("unknown database sub-command: %s", subCommand)
	}
}

// listDatabasesCommand lists all databases
func (c *CLI) listDatabasesCommand(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.ListDatabases(ctx, &pb.ListDatabasesRequest{
		Auth: &pb.AuthInfo{Password: c.password},
	})

	if err != nil {
		return fmt.Errorf("failed to list databases: %v", err)
	}

	if len(resp.Names) == 0 {
		fmt.Println("No databases found.")
	} else {
		fmt.Println("Databases:")
		for i, name := range resp.Names {
			fmt.Printf("%d) %s\n", i+1, name)
		}
	}

	return nil
}

// createDatabaseCommand creates a new database
func (c *CLI) createDatabaseCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: create-database <name>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.CreateDatabase(ctx, &pb.CreateDatabaseRequest{
		Auth: &pb.AuthInfo{Password: c.password},
		Name: args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}

	fmt.Printf("Database '%s' created successfully.\n", args[0])
	return nil
}

// dropDatabaseCommand drops a database
func (c *CLI) dropDatabaseCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: drop-database <name>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.DropDatabase(ctx, &pb.DropDatabaseRequest{
		Auth: &pb.AuthInfo{Password: c.password},
		Name: args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to drop database: %v", err)
	}

	fmt.Printf("Database '%s' dropped successfully.\n", args[0])
	return nil
}
