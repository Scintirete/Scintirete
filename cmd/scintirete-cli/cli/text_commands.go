package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

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
			return fmt.Errorf("usage: text insert <collection> [model] <id> <text> [metadata]")
		}
		return c.textInsertCommand(subArgs)
	case "search":
		if len(subArgs) < 2 {
			return fmt.Errorf("usage: text search <collection> [model] <text> [top-k] [ef-search]")
		}
		return c.textSearchCommand(subArgs)
	default:
		return fmt.Errorf("unknown text sub-command: %s", subCommand)
	}
}

// textInsertCommand handles text insertion with embedding
func (c *CLI) textInsertCommand(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: text insert <collection> [model] <id> <text> [metadata]")
	}

	collection := args[0]

	// Check if second argument looks like a model (doesn't start with digit or special chars typically used for IDs)
	var model string
	var id, text string
	var startIdx int

	if len(args) >= 4 && !strings.HasPrefix(args[1], "doc") && !strings.HasPrefix(args[1], "id") &&
		!strings.Contains(args[1], "-") && !strings.Contains(args[1], "_") && len(args[1]) > 10 {
		// Assume second argument is model
		model = args[1]
		id = args[2]
		text = args[3]
		startIdx = 4
	} else {
		// No model specified, use defaults
		id = args[1]
		text = args[2]
		startIdx = 3
	}

	// Parse optional metadata
	var metadata map[string]interface{}
	if len(args) >= startIdx+1 && args[startIdx] != "" {
		if err := json.Unmarshal([]byte(args[startIdx]), &metadata); err != nil {
			return fmt.Errorf("invalid metadata JSON: %v", err)
		}
	}

	// Convert string ID to uint64 pointer (nil means auto-generate)
	var numericId *uint64
	if id != "auto" && id != "" {
		var err error
		val, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID format: %v. Use a number or 'auto' for auto-generation", err)
		}
		numericId = &val
	}

	// Create the request
	req := &pb.EmbedAndInsertRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
		DbName:         currentDatabase,
		CollectionName: collection,
		Texts: []*pb.TextWithMetadata{
			{
				Id:   numericId,
				Text: text,
			},
		},
	}

	if metadata != nil {
		metadataStruct, err := ConvertToStruct(metadata)
		if err != nil {
			return fmt.Errorf("failed to convert metadata: %v", err)
		}
		req.Texts[0].Metadata = metadataStruct
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
		return fmt.Errorf("usage: text search <collection> [model] <text> [top-k] [ef-search]")
	}

	collection := args[0]

	// Check if second argument looks like a model name (longer text, no quotes)
	var model string
	var text string
	var startIdx int

	if len(args) >= 3 && len(args[1]) > 10 && !strings.HasPrefix(args[1], "\"") &&
		!strings.Contains(args[1], " ") {
		// Assume second argument is model
		model = args[1]
		text = args[2]
		startIdx = 3
	} else {
		// No model specified, use defaults
		text = args[1]
		startIdx = 2
	}

	// Parse optional top-k
	topK := int32(10) // default
	if len(args) >= startIdx+1 {
		k, err := strconv.Atoi(args[startIdx])
		if err != nil {
			return fmt.Errorf("invalid top-k value: %v", err)
		}
		topK = int32(k)
	}

	// Parse optional ef-search
	var efSearch *int32
	if len(args) >= startIdx+2 {
		ef, err := strconv.Atoi(args[startIdx+1])
		if err != nil {
			return fmt.Errorf("invalid ef-search value: %v", err)
		}
		efSearch = &[]int32{int32(ef)}[0]
	}

	// Create the request
	req := &pb.EmbedAndSearchRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
		DbName:         currentDatabase,
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
		fmt.Printf("%d. ID: %d, Distance: %.6f\n", i+1, result.Id, result.Distance)
		if result.Metadata != nil {
			metadata := ConvertFromStruct(result.Metadata)
			metadataJSON, _ := json.MarshalIndent(metadata, "   ", "  ")
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
