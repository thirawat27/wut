package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDatabasePath(t *testing.T) {
	t.Run("preserves existing file", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "legacy")
		if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if got := ResolveDatabasePath(file); got != file {
			t.Fatalf("expected existing file path %q, got %q", file, got)
		}
	})

	t.Run("resolves existing directory to wut db", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "dbdir")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		want := filepath.Join(dir, "wut.db")
		if got := ResolveDatabasePath(dir); got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("preserves explicit db extension", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "custom.db")
		if got := ResolveDatabasePath(path); got != path {
			t.Fatalf("expected %q, got %q", path, got)
		}
	})

	t.Run("normalizes missing legacy directory-like path", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "data")
		want := filepath.Join(base, "wut.db")
		if got := ResolveDatabasePath(base); got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestGetDatabaseAndTLDRPath(t *testing.T) {
	previousConfig := globalConfig
	t.Cleanup(func() {
		globalConfig = previousConfig
	})

	base := filepath.Join(t.TempDir(), "cache")
	globalConfig = &Config{
		Database: DatabaseConfig{
			Path: base,
		},
	}

	wantDB := filepath.Join(base, "wut.db")
	if got := GetDatabasePath(); got != wantDB {
		t.Fatalf("expected database path %q, got %q", wantDB, got)
	}

	wantTLDR := filepath.Join(base, "tldr.db")
	if got := GetTLDRDatabasePath(); got != wantTLDR {
		t.Fatalf("expected TLDR path %q, got %q", wantTLDR, got)
	}
}
