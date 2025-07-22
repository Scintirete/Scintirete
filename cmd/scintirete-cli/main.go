// Package main provides the command-line interface for Scintirete.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// CLI represents the command-line interface
type CLI struct {
	client   pb.ScintireteServiceClient
	conn     *grpc.ClientConn
	password string
	prompt   string
}

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func(*CLI, []string) error
}

func makeCommands() map[string]Command {
	return map[string]Command{
		"help":              {Name: "help", Description: "Show help information", Usage: "help [command]", Handler: (*CLI).helpCommand},
		"quit":              {Name: "quit", Description: "Exit the CLI", Usage: "quit", Handler: (*CLI).quitCommand},
		"exit":              {Name: "exit", Description: "Exit the CLI", Usage: "exit", Handler: (*CLI).quitCommand},
		"ping":              {Name: "ping", Description: "Test connection to server", Usage: "ping", Handler: (*CLI).pingCommand},
		"version":           {Name: "version", Description: "Show version information", Usage: "version", Handler: (*CLI).versionCommand},
		"list-databases":    {Name: "list-databases", Description: "List all databases", Usage: "list-databases", Handler: (*CLI).listDatabasesCommand},
		"create-database":   {Name: "create-database", Description: "Create a new database", Usage: "create-database <n>", Handler: (*CLI).createDatabaseCommand},
		"drop-database":     {Name: "drop-database", Description: "Drop a database", Usage: "drop-database <n>", Handler: (*CLI).dropDatabaseCommand},
		"use":               {Name: "use", Description: "Switch to a database", Usage: "use <database>", Handler: (*CLI).useCommand},
		"list-collections":  {Name: "list-collections", Description: "List collections in current database", Usage: "list-collections", Handler: (*CLI).listCollectionsCommand},
		"create-collection": {Name: "create-collection", Description: "Create a new collection", Usage: "create-collection <n> <metric> [hnsw-params]", Handler: (*CLI).createCollectionCommand},
		"drop-collection":   {Name: "drop-collection", Description: "Drop a collection", Usage: "drop-collection <n>", Handler: (*CLI).dropCollectionCommand},
		"collection-info":   {Name: "collection-info", Description: "Get collection information", Usage: "collection-info <n>", Handler: (*CLI).collectionInfoCommand},
		"insert":            {Name: "insert", Description: "Insert vectors into a collection", Usage: "insert <collection> <id> <vector> [metadata]", Handler: (*CLI).insertCommand},
		"search":            {Name: "search", Description: "Search for similar vectors", Usage: "search <collection> <vector> [top-k] [ef-search]", Handler: (*CLI).searchCommand},
		"delete":            {Name: "delete", Description: "Delete vectors from a collection", Usage: "delete <collection> <id1> [id2] ...", Handler: (*CLI).deleteCommand},
		"text":              {Name: "text", Description: "Text embedding operations", Usage: "text <insert|search> <args...>", Handler: (*CLI).textCommand},
		"save":              {Name: "save", Description: "Synchronously save RDB snapshot", Usage: "save", Handler: (*CLI).saveCommand},
		"bgsave":            {Name: "bgsave", Description: "Asynchronously save RDB snapshot", Usage: "bgsave", Handler: (*CLI).bgsaveCommand},
	}
}

var (
	version = "dev"
	commit  = "unknown"

	// Command line flags
	host     = flag.String("h", "localhost", "Server host")
	port     = flag.Int("p", 9090, "Server port")
	password = flag.String("a", "", "Password for authentication")
	database = flag.String("d", "", "Database name")
	help     = flag.Bool("help", false, "Show help")

	// Commands registry
	commands = makeCommands()
)

func main() {
	flag.Parse()

	if *help {
		showUsage()
		return
	}

	// Create CLI instance
	cli := &CLI{
		password: *password,
		prompt:   "scintirete> ",
	}

	// Connect to server
	if err := cli.connect(*host, *port); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to server: %v\n", err)
		os.Exit(1)
	}
	defer cli.close()

	// Set database if specified
	if *database != "" {
		cli.prompt = fmt.Sprintf("scintirete[%s]> ", *database)
	}

	// Check if there are command line arguments to execute
	args := flag.Args()
	if len(args) > 0 {
		// Execute single command and exit
		if err := cli.executeCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start interactive mode
	cli.interactive()
}

// connect establishes connection to the gRPC server
func (c *CLI) connect(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.conn = conn
	c.client = pb.NewScintireteServiceClient(conn)

	return nil
}

// close closes the connection
func (c *CLI) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// interactive starts the interactive mode
func (c *CLI) interactive() {
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

		args := parseCommand(line)
		if err := c.executeCommand(args); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// executeCommand executes a command with given arguments
func (c *CLI) executeCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmdName := strings.ToLower(args[0])
	cmd, exists := commands[cmdName]
	if !exists {
		return fmt.Errorf("unknown command: %s. Type 'help' for available commands", cmdName)
	}

	return cmd.Handler(c, args[1:])
}

// parseCommand parses a command line into arguments
func parseCommand(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// Command handlers

func (c *CLI) helpCommand(args []string) error {
	commands := makeCommands()
	if len(args) == 0 {
		fmt.Println("Available commands:")
		fmt.Println()
		for _, cmd := range commands {
			fmt.Printf("  %-20s %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println()
		fmt.Println("Type 'help <command>' for detailed usage information.")
	} else {
		cmdName := strings.ToLower(args[0])
		if cmd, exists := commands[cmdName]; exists {
			fmt.Printf("%s - %s\n", cmd.Name, cmd.Description)
			fmt.Printf("Usage: %s\n", cmd.Usage)
		} else {
			return fmt.Errorf("unknown command: %s", cmdName)
		}
	}
	return nil
}

func (c *CLI) quitCommand(args []string) error {
	fmt.Println("Goodbye!")
	os.Exit(0)
	return nil
}

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

func (c *CLI) versionCommand(args []string) error {
	fmt.Printf("Scintirete CLI %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	return nil
}

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

func (c *CLI) useCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: use <database>")
	}

	*database = args[0]
	c.prompt = fmt.Sprintf("scintirete[%s]> ", *database)
	fmt.Printf("Switched to database '%s'.\n", *database)
	return nil
}

func (c *CLI) listCollectionsCommand(args []string) error {
	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.ListCollections(ctx, &pb.ListCollectionsRequest{
		Auth:   &pb.AuthInfo{Password: c.password},
		DbName: *database,
	})

	if err != nil {
		return fmt.Errorf("failed to list collections: %v", err)
	}

	if len(resp.Collections) == 0 {
		fmt.Println("No collections found.")
	} else {
		fmt.Println("Collections:")
		for i, coll := range resp.Collections {
			fmt.Printf("%d) %s (dimension: %d, vectors: %d, metric: %s)\n",
				i+1, coll.Name, coll.Dimension, coll.VectorCount, coll.MetricType.String())
		}
	}

	return nil
}

func (c *CLI) createCollectionCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: create-collection <name> <metric> [m] [ef_construction]")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	name := args[0]
	metricStr := strings.ToUpper(args[1])

	var metric pb.DistanceMetric
	switch metricStr {
	case "L2", "EUCLIDEAN":
		metric = pb.DistanceMetric_L2
	case "COSINE":
		metric = pb.DistanceMetric_COSINE
	case "INNER_PRODUCT", "IP":
		metric = pb.DistanceMetric_INNER_PRODUCT
	default:
		return fmt.Errorf("invalid metric: %s. Use L2, COSINE, or INNER_PRODUCT", metricStr)
	}

	req := &pb.CreateCollectionRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: name,
		MetricType:     metric,
	}

	// Parse optional HNSW parameters
	if len(args) >= 3 {
		m, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid M parameter: %s", args[2])
		}

		efConstruction := 200 // default
		if len(args) >= 4 {
			efConstruction, err = strconv.Atoi(args[3])
			if err != nil {
				return fmt.Errorf("invalid ef_construction parameter: %s", args[3])
			}
		}

		req.HnswConfig = &pb.HnswConfig{
			M:              int32(m),
			EfConstruction: int32(efConstruction),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.CreateCollection(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %v", err)
	}

	fmt.Printf("Collection '%s' created successfully.\n", name)
	return nil
}

func (c *CLI) dropCollectionCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: drop-collection <name>")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.DropCollection(ctx, &pb.DropCollectionRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to drop collection: %v", err)
	}

	fmt.Printf("Collection '%s' dropped successfully.\n", args[0])
	return nil
}

func (c *CLI) collectionInfoCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: collection-info <name>")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.GetCollectionInfo(ctx, &pb.GetCollectionInfoRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to get collection info: %v", err)
	}

	fmt.Printf("Collection: %s\n", resp.Name)
	fmt.Printf("Dimension: %d\n", resp.Dimension)
	fmt.Printf("Vector Count: %d\n", resp.VectorCount)
	fmt.Printf("Deleted Count: %d\n", resp.DeletedCount)
	fmt.Printf("Memory Usage: %d bytes\n", resp.MemoryBytes)
	fmt.Printf("Distance Metric: %s\n", resp.MetricType.String())
	if resp.HnswConfig != nil {
		fmt.Printf("HNSW Config: M=%d, EfConstruction=%d\n", resp.HnswConfig.M, resp.HnswConfig.EfConstruction)
	}

	return nil
}

func (c *CLI) insertCommand(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: insert <collection> <id> <vector> [metadata]")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	collection := args[0]
	id := args[1]
	vectorStr := args[2]

	// Parse vector (JSON array format)
	var vector []float32
	if err := json.Unmarshal([]byte(vectorStr), &vector); err != nil {
		return fmt.Errorf("invalid vector format: %v. Use JSON array format: [1.0, 2.0, 3.0]", err)
	}

	pbVector := &pb.Vector{
		Id:       id,
		Elements: vector,
	}

	// Parse metadata if provided
	if len(args) >= 4 {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(args[3]), &metadata); err != nil {
			return fmt.Errorf("invalid metadata format: %v. Use JSON object format", err)
		}
		// Convert to protobuf Struct would require additional conversion
		// For now, skip metadata in CLI
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := c.client.InsertVectors(ctx, &pb.InsertVectorsRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: collection,
		Vectors:        []*pb.Vector{pbVector},
	})

	if err != nil {
		return fmt.Errorf("failed to insert vector: %v", err)
	}

	fmt.Printf("Vector '%s' inserted successfully.\n", id)
	return nil
}

func (c *CLI) searchCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: search <collection> <vector> [top-k] [ef-search]")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	collection := args[0]
	vectorStr := args[1]

	// Parse vector
	var vector []float32
	if err := json.Unmarshal([]byte(vectorStr), &vector); err != nil {
		return fmt.Errorf("invalid vector format: %v. Use JSON array format: [1.0, 2.0, 3.0]", err)
	}

	topK := int32(10) // default
	if len(args) >= 3 {
		k, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid top-k value: %s", args[2])
		}
		topK = int32(k)
	}

	req := &pb.SearchRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: collection,
		QueryVector:    vector,
		TopK:           topK,
	}

	if len(args) >= 4 {
		efSearch, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid ef-search value: %s", args[3])
		}
		efSearchInt32 := int32(efSearch)
		req.EfSearch = &efSearchInt32
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := c.client.Search(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to search: %v", err)
	}
	duration := time.Since(start)

	fmt.Printf("Search completed in %.2fms, found %d results:\n", float64(duration.Nanoseconds())/1e6, len(resp.Results))
	fmt.Println()

	for i, result := range resp.Results {
		fmt.Printf("%d) ID: %s, Distance: %.6f\n", i+1, result.Vector.Id, result.Distance)
		if len(result.Vector.Elements) > 0 {
			fmt.Printf("   Vector: [%.3f", result.Vector.Elements[0])
			for j := 1; j < len(result.Vector.Elements) && j < 5; j++ {
				fmt.Printf(", %.3f", result.Vector.Elements[j])
			}
			if len(result.Vector.Elements) > 5 {
				fmt.Printf(", ... (%d more)", len(result.Vector.Elements)-5)
			}
			fmt.Println("]")
		}
		fmt.Println()
	}

	return nil
}

func (c *CLI) deleteCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: delete <collection> <id1> [id2] ...")
	}

	if *database == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	collection := args[0]
	ids := args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.client.DeleteVectors(ctx, &pb.DeleteVectorsRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         *database,
		CollectionName: collection,
		Ids:            ids,
	})

	if err != nil {
		return fmt.Errorf("failed to delete vectors: %v", err)
	}

	fmt.Printf("Successfully deleted %d vectors.\n", resp.DeletedCount)
	return nil
}

// textCommand handles text embedding operations (insert and search)
func (c *CLI) textCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: text <insert|search> <args...>")
	}

	subCommand := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subCommand {
	case "insert":
		if len(subArgs) < 3 {
			return fmt.Errorf("usage: text insert <collection> <id> <text> [metadata] [model]")
		}
		return c.textInsertCommand(subArgs)
	case "search":
		if len(subArgs) < 2 {
			return fmt.Errorf("usage: text search <collection> <text> [top-k] [ef-search] [model]")
		}
		return c.textSearchCommand(subArgs)
	default:
		return fmt.Errorf("unknown text sub-command: %s", subCommand)
	}
}

// textInsertCommand handles text insertion with embedding
func (c *CLI) textInsertCommand(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: text insert <collection> <id> <text> [metadata] [model]")
	}

	collection := args[0]
	id := args[1]
	text := args[2]

	// Parse optional metadata
	var metadata map[string]interface{}
	if len(args) >= 4 && args[3] != "" {
		if err := json.Unmarshal([]byte(args[3]), &metadata); err != nil {
			return fmt.Errorf("invalid metadata JSON: %v", err)
		}
	}

	// Parse optional model
	var model string
	if len(args) >= 5 {
		model = args[4]
	}

	// Create the request
	req := &pb.EmbedAndInsertRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
		DbName:         *database,
		CollectionName: collection,
		Texts: []*pb.TextWithMetadata{
			{
				Id:   id,
				Text: text,
			},
		},
	}

	if metadata != nil {
		req.Texts[0].Metadata = convertToStruct(metadata)
	}

	if model != "" {
		req.EmbeddingModel = &model
	}

	// Make the request
	_, err := c.client.EmbedAndInsert(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to insert with embedding: %v", err)
	}

	fmt.Printf("Successfully inserted text with ID '%s' into collection '%s'\n", id, collection)
	return nil
}

// textSearchCommand handles text search with embedding
func (c *CLI) textSearchCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: text search <collection> <text> [top-k] [ef-search] [model]")
	}

	collection := args[0]
	text := args[1]

	// Parse optional top-k
	topK := int32(10) // default
	if len(args) >= 3 {
		k, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid top-k value: %v", err)
		}
		topK = int32(k)
	}

	// Parse optional ef-search
	var efSearch *int32
	if len(args) >= 4 {
		ef, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid ef-search value: %v", err)
		}
		efSearch = &[]int32{int32(ef)}[0]
	}

	// Parse optional model
	var model string
	if len(args) >= 5 {
		model = args[4]
	}

	// Create the request
	req := &pb.EmbedAndSearchRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
		DbName:         *database,
		CollectionName: collection,
		QueryText:      text,
		TopK:           topK,
		EfSearch:       efSearch,
	}

	if model != "" {
		req.EmbeddingModel = &model
	}

	// Make the request
	resp, err := c.client.EmbedAndSearch(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to search with embedding: %v", err)
	}

	// Display results
	fmt.Printf("Search results for text: \"%s\"\n", text)
	fmt.Printf("Found %d results:\n\n", len(resp.Results))

	for i, result := range resp.Results {
		fmt.Printf("%d. ID: %s, Distance: %.6f\n", i+1, result.Id, result.Distance)
		if result.Metadata != nil {
			metadataJSON, _ := json.MarshalIndent(convertFromStruct(result.Metadata), "   ", "  ")
			fmt.Printf("   Metadata: %s\n", string(metadataJSON))
		}
		if result.Vector != nil && len(result.Vector.Elements) > 0 {
			fmt.Printf("   Vector: [%.3f, %.3f, ...] (%d dimensions)\n",
				result.Vector.Elements[0],
				result.Vector.Elements[1],
				len(result.Vector.Elements))
		}
		fmt.Println()
	}

	return nil
}

// Helper function to convert map to protobuf Struct
func convertToStruct(m map[string]interface{}) *structpb.Struct {
	// For simplicity, we'll just return nil here
	// In a real implementation, you'd convert the map to a protobuf Struct
	return nil
}

// Helper function to convert protobuf Struct to map
func convertFromStruct(s *structpb.Struct) map[string]interface{} {
	// For simplicity, we'll just return an empty map here
	// In a real implementation, you'd convert the protobuf Struct to a map
	return make(map[string]interface{})
}

// saveCommand handles the save command
func (c *CLI) saveCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: save")
	}

	// Create the request
	req := &pb.SaveRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
	}

	// Make the request
	resp, err := c.client.Save(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to save RDB snapshot: %v", err)
	}

	if resp.Success {
		fmt.Printf("RDB snapshot saved successfully\n")
		fmt.Printf("Duration: %.3f seconds\n", resp.DurationSeconds)
		if resp.SnapshotSize > 0 {
			fmt.Printf("Snapshot size: %d bytes (%.2f MB)\n", resp.SnapshotSize, float64(resp.SnapshotSize)/(1024*1024))
		}
	} else {
		fmt.Printf("Failed to save RDB snapshot: %s\n", resp.Message)
	}

	return nil
}

// bgsaveCommand handles the bgsave command
func (c *CLI) bgsaveCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: bgsave")
	}

	// Create the request
	req := &pb.BgSaveRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
	}

	// Make the request
	resp, err := c.client.BgSave(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to start background RDB save: %v", err)
	}

	if resp.Success {
		fmt.Printf("Background RDB save started successfully\n")
		fmt.Printf("Job ID: %s\n", resp.JobId)
		fmt.Printf("Note: Check server logs for completion status\n")
	} else {
		fmt.Printf("Failed to start background RDB save: %s\n", resp.Message)
	}

	return nil
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
