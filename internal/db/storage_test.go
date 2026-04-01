package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()

	storage, err := NewStorage(filepath.Join(t.TempDir(), "wut.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() {
		_ = storage.Close()
	})

	return storage
}

func TestStoragePlatformQueriesAndCommandListing(t *testing.T) {
	storage := newTestStorage(t)

	pages := []*Page{
		{Name: "git", Platform: PlatformCommon, Language: "en", Description: "Git basics"},
		{Name: "tar", Platform: PlatformLinux, Language: "en", Description: "Archive files"},
		{Name: "tar", Platform: PlatformWindows, Language: "en", Description: "Archive files on Windows"},
		{Name: "grep", Platform: PlatformLinux, Language: "fr", Description: "Rechercher du texte"},
	}
	if err := storage.SavePages(pages); err != nil {
		t.Fatalf("save pages: %v", err)
	}

	linuxPages, err := storage.GetPagesByPlatform(PlatformLinux)
	if err != nil {
		t.Fatalf("get pages by platform: %v", err)
	}
	if len(linuxPages) != 2 {
		t.Fatalf("expected 2 linux pages, got %d", len(linuxPages))
	}

	commands, err := storage.ListCommands(0)
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	wantCommands := []string{"git", "grep", "tar"}
	if !reflect.DeepEqual(commands, wantCommands) {
		t.Fatalf("expected commands %v, got %v", wantCommands, commands)
	}

	results, err := storage.SearchLocalLimited("archive", 1)
	if err != nil {
		t.Fatalf("search local limited: %v", err)
	}
	if len(results) != 1 || results[0].Name != "tar" {
		t.Fatalf("expected one tar result, got %+v", results)
	}
}

func TestGetPageAnyPlatformFallsBackToEnglish(t *testing.T) {
	storage := newTestStorage(t)

	page := &Page{
		Name:        "powershell",
		Platform:    PlatformWindows,
		Language:    "en",
		Description: "PowerShell commands",
	}
	if err := storage.SavePage(page); err != nil {
		t.Fatalf("save page: %v", err)
	}

	got, err := storage.GetPageAnyPlatform("powershell", "fr")
	if err != nil {
		t.Fatalf("get page any platform: %v", err)
	}
	if got.Platform != PlatformWindows {
		t.Fatalf("expected platform %q, got %q", PlatformWindows, got.Platform)
	}
	if got.Language != "en" {
		t.Fatalf("expected english fallback, got %q", got.Language)
	}
}
