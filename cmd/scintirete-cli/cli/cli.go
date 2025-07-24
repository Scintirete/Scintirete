package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// connect establishes connection to the gRPC server
func (c *CLI) Connect(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.conn = conn
	c.client = pb.NewScintireteServiceClient(conn)

	return nil
}

// Close closes the connection
func (c *CLI) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Interactive starts the interactive mode
func (c *CLI) Interactive(version, commit string) {
	fmt.Printf("Scintirete CLI %s (commit: %s)\n", version, commit)
	fmt.Println("Type 'help' for available commands or 'quit' to exit.")
	fmt.Println()

	rl, err := readline.New(c.prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			}
			continue
		} else if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		args := ParseCommand(line)
		if err := c.ExecuteCommand(args); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// ExecuteCommand executes a command with given arguments
func (c *CLI) ExecuteCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmdName := strings.ToLower(args[0])
	cmd, exists := GetCommands()[cmdName]
	if !exists {
		return fmt.Errorf("unknown command: %s. Type 'help' for available commands", cmdName)
	}

	return cmd.Handler(c, args[1:])
}
