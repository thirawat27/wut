// Package db provides TLDR Pages storage for offline access
package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"go.etcd.io/bbolt"
)

const (
	tldrBucketName = "tldr_pages"
	metadataBucket = "tldr_metadata"
)

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

// Metadata stores sync information
type Metadata struct {
	LastSync   time.Time `json:"last_sync"`
	TotalPages int       `json:"total_pages"`
	Platforms  []string  `json:"platforms"`
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

	lang := page.Language
	if lang == "" {
		lang = "en"
	}
	key := fmt.Sprintf("%s/%s/%s", lang, page.Platform, page.Name)

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

			lang := page.Language
			if lang == "" {
				lang = "en"
			}
			key := fmt.Sprintf("%s/%s/%s", lang, page.Platform, page.Name)
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

	key := fmt.Sprintf("%s/%s/%s", language, platform, name)

	var stored StoredPage
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		data := bucket.Get([]byte(key))

		// Fallback to English if not found
		if data == nil && language != "en" {
			fallbackKey := fmt.Sprintf("en/%s/%s", platform, name)
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

	for _, platform := range platforms {
		page, err := s.GetPage(name, platform, language)
		if err == nil {
			return page, nil
		}
	}

	return nil, fmt.Errorf("page not found in local storage: %s", name)
}

// PageExists checks if a page exists in local storage
func (s *Storage) PageExists(name, platform, language string) bool {
	if language == "" {
		language = "en"
	}
	key := fmt.Sprintf("%s/%s/%s", language, platform, name)
	exists := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		exists = bucket.Get([]byte(key)) != nil
		return nil
	})

	return exists && err == nil
}

// IsPageStale checks if a page is older than the given duration
func (s *Storage) IsPageStale(name, platform, language string, maxAge time.Duration) bool {
	if language == "" {
		language = "en"
	}
	key := fmt.Sprintf("%s/%s/%s", language, platform, name)
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

// GetPagesByPlatform returns all pages for a specific platform
func (s *Storage) GetPagesByPlatform(platform string) ([]StoredPage, error) {
	var pages []StoredPage
	prefix := platform + "/"

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.ForEach(func(k, v []byte) error {
			if len(k) > len(prefix) && string(k[:len(prefix)]) == prefix {
				var stored StoredPage
				if err := json.Unmarshal(v, &stored); err == nil {
					pages = append(pages, stored)
				}
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
	key := fmt.Sprintf("%s/%s/%s", language, platform, name)
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(tldrBucketName))
		return bucket.Delete([]byte(key))
	})
}

// ClearAll removes all pages from local storage
func (s *Storage) ClearAll() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket([]byte(tldrBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucket([]byte(tldrBucketName)); err != nil {
			return err
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

	pages, err := s.GetAllPages()
	if err != nil {
		return nil, err
	}

	platforms := map[string]int{}
	for _, page := range pages {
		platforms[page.Platform]++
	}

	stats["total_pages"] = len(pages)
	stats["platforms"] = platforms

	// Get last sync
	if meta, err := s.GetMetadata(); err == nil {
		stats["last_sync"] = meta.LastSync
	}

	return stats, nil
}

// SearchLocal searches pages in local storage by name or description
func (s *Storage) SearchLocal(query string) ([]StoredPage, error) {
	var results []StoredPage
	queryLower := strings.ToLower(query)

	pages, err := s.GetAllPages()
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		nameLower := strings.ToLower(page.Name)
		descLower := strings.ToLower(page.Description)

		if strings.Contains(nameLower, queryLower) || strings.Contains(descLower, queryLower) {
			results = append(results, page)
		}
	}

	return results, nil
}
