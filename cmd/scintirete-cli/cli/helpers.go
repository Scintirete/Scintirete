package cli

import (
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

// parseCommand parses a command line into arguments
func ParseCommand(line string) []string {
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

// convertToStruct converts a map to protobuf Struct
func ConvertToStruct(m map[string]interface{}) (*structpb.Struct, error) {
	if m == nil {
		return nil, nil
	}

	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// convertFromStruct converts protobuf Struct to map
func ConvertFromStruct(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return make(map[string]interface{})
	}
	return s.AsMap()
}
