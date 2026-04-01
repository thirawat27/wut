// Package db provides TLDR Pages storage for offline access
package db

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"go.etcd.io/bbolt"
)

const (
	tldrBucketName = "tldr_pages"
	metadataBucket = "tldr_metadata"
)

var errStopScan = errors.New("stop scan")

// Storage provides local storage for TLDR pages
type Storage struct {
	db   *bbolt.DB
	path string
}

// StoredPage represents a TLDR page stored locally
type StoredPage struct {
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Language    string    `json:"language"`
	Description string    `json:"description"`
	Examples    []Example `json:"examples"`
	RawContent  string    `json:"raw_content"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// PageRef identifies a specific stored TLDR page variant.
type PageRef struct {
	Name     string
	Platform string
	Language string
}

// Metadata stores sync information
type Metadata struct {
	LastSync   time.Time `json:"last_sync"`
	TotalPages int       `json:"total_pages"`
	Platforms  []string  `json:"platforms"`
}

type storedPageSummary struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

type storedPageTimestamp struct {
	FetchedAt time.Time `json:"fetched_at"`
}

func pageKey(language, platform, name string) string {
	if language == "" {
		language = "en"
	}
	return fmt.Sprintf("%s/%s/%s", language, platform, name)
}

func parsePageKey(key []byte) (language, platform, name string, ok bool) {
	parts := strings.SplitN(string(key), "/", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func summaryToStoredPage(summary storedPageSummary) StoredPage {
	return StoredPage{
		Name:        summary.Name,
		Platform:    summary.Platform,
		Language:    summary.Language,
		Description: summary.Description,
	}
}

// NewStorage creates a new TLDR storage
func NewStorage(dbPath string) (*Storage, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(tldrBucketName)); err != nil {
			return fmt.Errorf("create tldr bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(metadataBucket)); err != nil {
			return fmt.Errorf("create metadata bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{
		db:   db,
		path: dbPath,
	}, nil
}

// Close closes the storage
func (s *Storage) Close() error {
	return s.db.Close()
}

// SavePage saves a TLDR page to local storage
func (s *Storage) SavePage(page *Page) error {
	stored := StoredPage{
		Name:        page.Name,
		Platform:    page.Platform,
		Language:    page.Language,
		Description: page.Description,
		Examples:    page.Examples,
		RawContent:  page.RawContent,
		FetchedAt:   time.Now(),
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("failed to marshal page: %w", err)
	}

	key := pageKey(page.Language, page.Platform, page.Name)

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.Put([]byte(key), data)
	})
}

// SavePages saves multiple TLDR pages to local storage in a single transaction
func (s *Storage) SavePages(pages []*Page) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		for _, page := range pages {
			stored := StoredPage{
				Name:        page.Name,
				Platform:    page.Platform,
				Language:    page.Language,
				Description: page.Description,
				Examples:    page.Examples,
				RawContent:  page.RawContent,
				FetchedAt:   time.Now(),
			}

			data, err := json.Marshal(stored)
			if err != nil {
				return fmt.Errorf("failed to marshal page %s: %w", page.Name, err)
			}

			key := pageKey(page.Language, page.Platform, page.Name)
			if err := bucket.Put([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetPage retrieves a TLDR page from local storage for a specific language
func (s *Storage) GetPage(name, platform, language string) (*Page, error) {
	if language == "" {
		language = "en"
	}

	key := pageKey(language, platform, name)

	var stored StoredPage
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		data := bucket.Get([]byte(key))

		// Fallback to English if not found
		if data == nil && language != "en" {
			fallbackKey := pageKey("en", platform, name)
			data = bucket.Get([]byte(fallbackKey))
		}

		if data == nil {
			return fmt.Errorf("page not found")
		}
		return json.Unmarshal(data, &stored)
	})
	if err != nil {
		return nil, err
	}

	return &Page{
		Name:        stored.Name,
		Platform:    stored.Platform,
		Language:    stored.Language,
		Description: stored.Description,
		Examples:    stored.Examples,
		RawContent:  stored.RawContent,
	}, nil
}

// GetPageAnyPlatform tries to get a page from any available platform in local storage
func (s *Storage) GetPageAnyPlatform(name, language string) (*Page, error) {
	if language == "" {
		language = "en"
	}

	platforms := []string{
		PlatformCommon,
		PlatformLinux,
		PlatformMacOS,
		PlatformWindows,
		PlatformFreeBSD,
		PlatformOpenBSD,
		PlatformNetBSD,
		PlatformSunOS,
		PlatformAndroid,
	}

	var stored StoredPage
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		languages := []string{language}
		if language != "en" {
			languages = append(languages, "en")
		}

		for _, lang := range languages {
			for _, platform := range platforms {
				data := bucket.Get([]byte(pageKey(lang, platform, name)))
				if data == nil {
					continue
				}
				return json.Unmarshal(data, &stored)
			}
		}

		return fmt.Errorf("page not found")
	})
	if err != nil {
		return nil, fmt.Errorf("page not found in local storage: %s", name)
	}

	return &Page{
		Name:        stored.Name,
		Platform:    stored.Platform,
		Language:    stored.Language,
		Description: stored.Description,
		Examples:    stored.Examples,
		RawContent:  stored.RawContent,
	}, nil
}

// PageExists checks if a page exists in local storage
func (s *Storage) PageExists(name, platform, language string) bool {
	if language == "" {
		language = "en"
	}
	key := pageKey(language, platform, name)
	exists := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		exists = bucket.Get([]byte(key)) != nil
		return nil
	})

	return exists && err == nil
}

// PageExistsAnyPlatform checks whether a command exists in local storage for
// any supported platform, falling back to English when needed.
func (s *Storage) PageExistsAnyPlatform(name, language string) bool {
	if language == "" {
		language = "en"
	}

	platforms := []string{
		PlatformCommon,
		PlatformLinux,
		PlatformMacOS,
		PlatformWindows,
		PlatformFreeBSD,
		PlatformOpenBSD,
		PlatformNetBSD,
		PlatformSunOS,
		PlatformAndroid,
	}

	exists := false

	_ = s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		languages := []string{language}
		if language != "en" {
			languages = append(languages, "en")
		}

		for _, lang := range languages {
			for _, platform := range platforms {
				if bucket.Get([]byte(pageKey(lang, platform, name))) != nil {
					exists = true
					return errStopScan
				}
			}
		}
		return nil
	})

	return exists
}

// IsPageStale checks if a page is older than the given duration
func (s *Storage) IsPageStale(name, platform, language string, maxAge time.Duration) bool {
	if language == "" {
		language = "en"
	}
	key := pageKey(language, platform, name)
	isStale := true

	_ = s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		data := bucket.Get([]byte(key))
		if data == nil {
			return nil
		}

		var stored StoredPage
		if err := json.Unmarshal(data, &stored); err != nil {
			return nil
		}

		isStale = time.Since(stored.FetchedAt) > maxAge
		return nil
	})

	return isStale
}

// GetAllPages returns all pages from local storage
func (s *Storage) GetAllPages() ([]StoredPage, error) {
	var pages []StoredPage

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			var stored StoredPage
			if err := json.Unmarshal(v, &stored); err == nil {
				pages = append(pages, stored)
			}
			return nil
		})
	})

	return pages, err
}

// GetPageSummaries returns page metadata without examples/raw content to reduce
// allocations in list/search flows.
func (s *Storage) GetPageSummaries(limit int) ([]StoredPage, error) {
	var pages []StoredPage

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			var summary storedPageSummary
			if err := json.Unmarshal(v, &summary); err == nil {
				pages = append(pages, summaryToStoredPage(summary))
				if limit > 0 && len(pages) >= limit {
					return errStopScan
				}
			}
			return nil
		})
	})
	if errors.Is(err, errStopScan) {
		err = nil
	}

	return pages, err
}

// ListCommands returns unique command names from the TLDR bucket without
// unmarshalling full page payloads.
func (s *Storage) ListCommands(limit int) ([]string, error) {
	seen := make(map[string]struct{})
	commands := make([]string, 0)

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			_, _, name, ok := parsePageKey(k)
			if !ok {
				return nil
			}
			if _, exists := seen[name]; exists {
				return nil
			}
			seen[name] = struct{}{}
			commands = append(commands, name)
			if limit > 0 && len(commands) >= limit {
				return errStopScan
			}
			return nil
		})
	})
	if errors.Is(err, errStopScan) {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	sort.Strings(commands)
	return commands, nil
}

// GetPagesByPlatform returns all pages for a specific platform
func (s *Storage) GetPagesByPlatform(platform string) ([]StoredPage, error) {
	var pages []StoredPage
	platform = strings.ToLower(strings.TrimSpace(platform))

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			_, keyPlatform, _, ok := parsePageKey(k)
			if !ok || keyPlatform != platform {
				return nil
			}
			var stored StoredPage
			if err := json.Unmarshal(v, &stored); err == nil {
				pages = append(pages, stored)
			}
			return nil
		})
	})

	return pages, err
}

// DeletePage deletes a page from local storage
func (s *Storage) DeletePage(name, platform, language string) error {
	if language == "" {
		language = "en"
	}
	key := pageKey(language, platform, name)
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.Delete([]byte(key))
	})
}

// ClearAll removes all pages from local storage
func (s *Storage) ClearAll() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		for _, bucketName := range []string{tldrBucketName, metadataBucket} {
			if err := tx.DeleteBucket([]byte(bucketName)); err != nil && !errors.Is(err, bbolt.ErrBucketNotFound) {
				return err
			}
			if _, err := tx.CreateBucket([]byte(bucketName)); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveMetadata saves metadata to storage
func (s *Storage) SaveMetadata(meta *Metadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		return bucket.Put([]byte("metadata"), data)
	})
}

// GetMetadata retrieves metadata from storage
func (s *Storage) GetMetadata() (*Metadata, error) {
	var meta Metadata
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		data := bucket.Get([]byte("metadata"))
		if data == nil {
			return fmt.Errorf("no metadata found")
		}
		return json.Unmarshal(data, &meta)
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetStats returns storage statistics
func (s *Storage) GetStats() (map[string]any, error) {
	stats := map[string]any{
		"total_pages": 0,
		"platforms":   map[string]int{},
	}

	platforms := map[string]int{}
	totalPages := 0

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			_, platform, _, ok := parsePageKey(k)
			if ok {
				totalPages++
				platforms[platform]++
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	stats["total_pages"] = totalPages
	stats["platforms"] = platforms

	// Get last sync
	if meta, err := s.GetMetadata(); err == nil {
		stats["last_sync"] = meta.LastSync
	}

	return stats, nil
}

// CountPages returns the total number of stored TLDR pages.
func (s *Storage) CountPages() (int, error) {
	totalPages := 0

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			if _, _, _, ok := parsePageKey(k); ok {
				totalPages++
			}
			return nil
		})
	})
	if err != nil {
		return 0, err
	}

	return totalPages, nil
}

// ListStalePages returns page variants older than maxAge.
func (s *Storage) ListStalePages(maxAge time.Duration, limit int) ([]PageRef, error) {
	stalePages := make([]PageRef, 0)
	now := time.Now()

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			language, platform, name, ok := parsePageKey(k)
			if !ok {
				return nil
			}

			var stored storedPageTimestamp
			if err := json.Unmarshal(v, &stored); err != nil {
				return nil
			}
			if now.Sub(stored.FetchedAt) <= maxAge {
				return nil
			}

			stalePages = append(stalePages, PageRef{
				Name:     name,
				Platform: platform,
				Language: language,
			})
			if limit > 0 && len(stalePages) >= limit {
				return errStopScan
			}
			return nil
		})
	})
	if errors.Is(err, errStopScan) {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	return stalePages, nil
}

// SearchLocal searches pages in local storage by name or description
func (s *Storage) SearchLocal(query string) ([]StoredPage, error) {
	return s.SearchLocalLimited(query, 0)
}

// SearchLocalLimited searches page metadata locally and optionally stops after
// `limit` matches to keep interactive search responsive.
func (s *Storage) SearchLocalLimited(query string, limit int) ([]StoredPage, error) {
	var results []StoredPage
	queryLower := strings.ToLower(strings.TrimSpace(query))

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			_, _, keyName, ok := parsePageKey(k)
			if !ok {
				return nil
			}

			var summary storedPageSummary
			if err := json.Unmarshal(v, &summary); err != nil {
				return nil
			}

			if queryLower == "" ||
				strings.Contains(strings.ToLower(keyName), queryLower) ||
				strings.Contains(strings.ToLower(summary.Description), queryLower) {
				results = append(results, summaryToStoredPage(summary))
				if limit > 0 && len(results) >= limit {
					return errStopScan
				}
			}
			return nil
		})
	})
	if errors.Is(err, errStopScan) {
		err = nil
	}

	return results, err
}
