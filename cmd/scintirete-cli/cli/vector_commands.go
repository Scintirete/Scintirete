package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// vectorCommand handles vector operations
func (c *CLI) vectorCommand(args []string) error {
	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: vector <insert|search|delete> [args...]")
	}

	subCommand := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subCommand {
	case "insert":
		if len(subArgs) < 3 {
			return fmt.Errorf("usage: vector insert <collection> <id> <vector> [metadata]")
		}
		return c.insertCommand(subArgs)
	case "search":
		if len(subArgs) < 2 {
			return fmt.Errorf("usage: vector search <collection> <vector> [top-k] [ef-search]")
		}
		return c.searchCommand(subArgs)
	case "delete":
		if len(subArgs) < 2 {
			return fmt.Errorf("usage: vector delete <collection> <id1> [id2] ...")
		}
		return c.deleteCommand(subArgs)
	default:
		return fmt.Errorf("unknown vector sub-command: %s", subCommand)
	}
}

// insertCommand inserts vectors
func (c *CLI) insertCommand(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: insert <collection> <id> <vector> [metadata]")
	}

	if currentDatabase == "" {
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

	// Convert string ID to uint64 (0 means auto-generate)
	var numericId uint64
	if id != "auto" && id != "" {
		var err error
		numericId, err = strconv.ParseUint(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID format: %v. Use a number or 'auto' for auto-generation", err)
		}
	}

	pbVector := &pb.Vector{
		Id:       numericId,
		Elements: vector,
	}

	// Parse metadata if provided
	if len(args) >= 4 {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(args[3]), &metadata); err != nil {
			return fmt.Errorf("invalid metadata format: %v. Use JSON object format", err)
		}

		metadataStruct, err := ConvertToStruct(metadata)
		if err != nil {
			return fmt.Errorf("failed to convert metadata: %v", err)
		}
		pbVector.Metadata = metadataStruct
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := c.client.InsertVectors(ctx, &pb.InsertVectorsRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         currentDatabase,
		CollectionName: collection,
		Vectors:        []*pb.Vector{pbVector},
	})

	if err != nil {
		return fmt.Errorf("failed to insert vector: %v", err)
	}

	fmt.Printf("Vector '%s' inserted successfully.\n", id)
	return nil
}

// searchCommand searches vectors
func (c *CLI) searchCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: search <collection> <vector> [top-k] [ef-search]")
	}

	if currentDatabase == "" {
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
		DbName:         currentDatabase,
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
		fmt.Printf("%d) ID: %d, Distance: %.6f\n", i+1, result.Vector.Id, result.Distance)
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

		// Display metadata if available
		if result.Vector.Metadata != nil {
			metadata := ConvertFromStruct(result.Vector.Metadata)
			if len(metadata) > 0 {
				metadataJSON, _ := json.MarshalIndent(metadata, "   ", "  ")
				fmt.Printf("   Metadata: %s\n", string(metadataJSON))
			}
		}
		fmt.Println()
	}

	return nil
}

// deleteCommand deletes vectors
func (c *CLI) deleteCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: delete <collection> <id1> [id2] ...")
	}

	if currentDatabase == "" {
		return fmt.Errorf("no database selected. Use 'use <database>' first")
	}

	collection := args[0]
	stringIds := args[1:]

	// Convert string IDs to uint64
	ids := make([]uint64, len(stringIds))
	for i, strId := range stringIds {
		id, err := strconv.ParseUint(strId, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID format '%s': %v. IDs must be numbers", strId, err)
		}
		ids[i] = id
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.client.DeleteVectors(ctx, &pb.DeleteVectorsRequest{
		Auth:           &pb.AuthInfo{Password: c.password},
		DbName:         currentDatabase,
		CollectionName: collection,
		Ids:            ids,
	})

	if err != nil {
		return fmt.Errorf("failed to delete vectors: %v", err)
	}

	fmt.Printf("Successfully deleted %d vectors.\n", resp.DeletedCount)
	return nil
}
