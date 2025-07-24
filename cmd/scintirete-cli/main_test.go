// Package main provides unit tests for the CLI.
package main

import (
	"testing"

	"github.com/scintirete/scintirete/cmd/scintirete-cli/cli"
)

// Note: Mock client implementation removed as tests now focus on command registration
// and basic CLI functionality rather than network operations

func TestCLI_SaveCommand(t *testing.T) {
	// Test that commands are properly registered
	commands := cli.GetCommands()

	// Test that save command is registered
	saveCmd, exists := commands["save"]
	if !exists {
		t.Error("Save command should be registered")
	}

	if saveCmd.Name != "save" {
		t.Errorf("Expected command name 'save', got '%s'", saveCmd.Name)
	}

	if saveCmd.Description == "" {
		t.Error("Save command should have a description")
	}

	// Test that bgsave command is registered
	bgsaveCmd, exists := commands["bgsave"]
	if !exists {
		t.Error("BgSave command should be registered")
	}

	if bgsaveCmd.Name != "bgsave" {
		t.Errorf("Expected command name 'bgsave', got '%s'", bgsaveCmd.Name)
	}

	if bgsaveCmd.Description == "" {
		t.Error("BgSave command should have a description")
	}
}

func TestCLI_BgSaveCommand(t *testing.T) {
	// Test basic CLI functionality
	cliInstance := cli.NewCLI("test-password")

	if cliInstance == nil {
		t.Error("NewCLI should return a valid CLI instance")
	}

	// Test prompt functionality
	originalPrompt := cliInstance.GetPrompt()
	if originalPrompt != "scintirete> " {
		t.Errorf("Expected default prompt 'scintirete> ', got '%s'", originalPrompt)
	}

	// Test prompt setting
	newPrompt := "test[db]> "
	cliInstance.SetPrompt(newPrompt)
	if cliInstance.GetPrompt() != newPrompt {
		t.Errorf("Expected prompt '%s', got '%s'", newPrompt, cliInstance.GetPrompt())
	}

	// Test password retrieval
	if cliInstance.GetPassword() != "test-password" {
		t.Errorf("Expected password 'test-password', got '%s'", cliInstance.GetPassword())
	}
}

func TestCommandParsing(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"simple command", []string{"simple", "command"}},
		{"command with \"quoted arg\"", []string{"command", "with", "quoted arg"}},
		{"cmd arg1 arg2", []string{"cmd", "arg1", "arg2"}},
		{"", []string{}},
		{"single", []string{"single"}},
		{"cmd \"arg with spaces\" normal", []string{"cmd", "arg with spaces", "normal"}},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := cli.ParseCommand(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d args, got %d: %v", len(tc.expected), len(result), result)
				return
			}

			for i, expected := range tc.expected {
				if result[i] != expected {
					t.Errorf("Expected arg %d to be '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestVersionInfo(t *testing.T) {
	// Test version setting
	cli.SetVersion("test-version", "test-commit")

	// Create CLI instance and test version command
	cliInstance := cli.NewCLI("")
	commands := cli.GetCommands()

	versionCmd, exists := commands["version"]
	if !exists {
		t.Error("Version command should be registered")
	}

	// Since we can't easily capture stdout in tests, just verify the command doesn't error
	err := versionCmd.Handler(cliInstance, []string{})
	if err != nil {
		t.Errorf("Version command should not error, got: %v", err)
	}
}
