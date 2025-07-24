package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// Current database (package level variable for simplicity)
var currentDatabase string

// collectionCommand handles collection operations
func (c *CLI) collectionCommand(args []string) error {
	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: collection <list|create|drop|info> [args...]")
	}

	subCommand := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subCommand {
	case "list":
		return c.listCollectionsCommand(subArgs)
	case "create":
		if len(subArgs) < 2 {
			return fmt.Errorf("usage: collection create <name> <metric> [m] [ef_construction]")
		}
		return c.createCollectionCommand(subArgs)
	case "drop":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: collection drop <name>")
		}
		return c.dropCollectionCommand(subArgs)
	case "info":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: collection info <name>")
		}
		return c.collectionInfoCommand(subArgs)
	default:
		return fmt.Errorf("unknown collection sub-command: %s", subCommand)
	}
}

// listCollectionsCommand lists collections in current database
func (c *CLI) listCollectionsCommand(args []string) error {
	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.ListCollections(ctx, &pb.ListCollectionsRequest{
		Auth:   &pb.AuthInfo{Password: c.password},
		DbName: currentDatabase,
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

// createCollectionCommand creates a collection
func (c *CLI) createCollectionCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: create-collection <name> <metric> [m] [ef_construction]")
	}

	if currentDatabase == "" {
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
		DbName:         currentDatabase,
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

// dropCollectionCommand drops a collection
func (c *CLI) dropCollectionCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: drop-collection <name>")
	}

	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.DropCollection(ctx, &pb.DropCollectionRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         currentDatabase,
		CollectionName: args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to drop collection: %v", err)
	}

	fmt.Printf("Collection '%s' dropped successfully.\n", args[0])
	return nil
}

// collectionInfoCommand gets collection information
func (c *CLI) collectionInfoCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: collection-info <name>")
	}

	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.GetCollectionInfo(ctx, &pb.GetCollectionInfoRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         currentDatabase,
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

// SetCurrentDatabase sets the current database
func SetCurrentDatabase(database string) {
	currentDatabase = database
}

// GetCurrentDatabase returns the current database
func GetCurrentDatabase() string {
	return currentDatabase
}
