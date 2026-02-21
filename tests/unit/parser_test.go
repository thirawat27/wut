package unit

import (
	"testing"
	"wut/internal/core"
)

func TestParseSimpleCommand(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("git status")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Command != "git" {
		t.Errorf("Expected command 'git', got '%s'", parsed.Command)
	}
	if parsed.Subcommand != "status" {
		t.Errorf("Expected subcommand 'status', got '%s'", parsed.Subcommand)
	}
}

func TestParseCommandWithFlags(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("git commit -m 'test message'")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Command != "git" {
		t.Errorf("Expected command 'git', got '%s'", parsed.Command)
	}
	if len(parsed.Flags) == 0 {
		t.Error("Expected flags to be parsed")
	}
}

func TestParsePipedCommand(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("cat file.txt | grep test")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !parsed.IsPipe {
		t.Error("Expected IsPipe to be true")
	}
	if len(parsed.PipedCommands) != 2 {
		t.Errorf("Expected 2 piped commands, got %d", len(parsed.PipedCommands))
	}
}

func TestParseRedirection(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("echo test > output.txt")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !parsed.IsRedirect {
		t.Error("Expected IsRedirect to be true")
	}
}

func TestParseEmptyCommand(t *testing.T) {
	p := core.NewParser()
	_, err := p.Parse("")
	
	if err == nil {
		t.Error("Expected error for empty command")
	}
}

func TestParseLongFlag(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("docker run --name myapp")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	
	foundFlag := false
	for _, flag := range parsed.Flags {
		if flag.Name == "name" && !flag.IsShort {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Error("Expected to find long flag '--name'")
	}
}

func TestParseShortFlag(t *testing.T) {
	p := core.NewParser()
	parsed, err := p.Parse("ls -la")
	
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(parsed.Flags) == 0 {
		t.Error("Expected flags to be parsed")
	}
}
