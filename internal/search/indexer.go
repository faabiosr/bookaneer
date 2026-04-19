package search

import (
	"context"
	"time"
)

// Indexer defines the interface for search indexers (Newznab, Torznab).
type Indexer interface {
	Name() string
	Type() string
	Search(ctx context.Context, query SearchQuery) ([]Result, error)
	Caps(ctx context.Context) (*Capabilities, error)
	Test(ctx context.Context) error
}

// SearchQuery represents a search request.
type SearchQuery struct {
	Query    string
	Author   string
	Title    string
	ISBN     string
	Category []string
	Limit    int
	Offset   int
}

// Result represents a single search result from an indexer.
type Result struct {
	GUID        string    `json:"guid"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size"`
	PubDate     time.Time `json:"pubDate"`
	Category    string    `json:"category,omitempty"`
	CategoryID  string    `json:"categoryId,omitempty"`
	DownloadURL string    `json:"downloadUrl"`
	InfoURL     string    `json:"infoUrl,omitempty"`
	Comments    int       `json:"comments,omitempty"`
	Seeders     int       `json:"seeders,omitempty"`
	Leechers    int       `json:"leechers,omitempty"`
	Grabs       int       `json:"grabs,omitempty"`
	Quality     string    `json:"quality,omitempty"`
	QualityRank int       `json:"qualityRank,omitempty"`
	IndexerID   int64     `json:"indexerId"`
	IndexerName string    `json:"indexerName"`
}

// Capabilities describes what an indexer can do.
type Capabilities struct {
	Searching struct {
		Search     bool `json:"search"`
		BookSearch bool `json:"bookSearch"`
	} `json:"searching"`
	Categories []Category `json:"categories"`
}

// Category represents a category supported by an indexer.
type Category struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	SubCategory []Category `json:"subCategory,omitempty"`
}

// IndexerConfig holds the configuration for an indexer.
type IndexerConfig struct {
	ID                      int64    `json:"id"                      db:"id"`
	Name                    string   `json:"name"                    db:"name"`
	Type                    string   `json:"type"                    db:"type"`
	BaseURL                 string   `json:"baseUrl"                 db:"base_url"`
	APIPath                 string   `json:"apiPath"                 db:"api_path"`
	APIKey                  string   `json:"apiKey"                  db:"api_key"`
	Categories              string   `json:"categories"              db:"categories"`
	Priority                int      `json:"priority"                db:"priority"`
	Enabled                 bool     `json:"enabled"                 db:"enabled"`
	EnableRSS               bool     `json:"enableRss"               db:"enable_rss"`
	EnableAutomaticSearch   bool     `json:"enableAutomaticSearch"   db:"enable_automatic_search"`
	EnableInteractiveSearch bool     `json:"enableInteractiveSearch" db:"enable_interactive_search"`
	AdditionalParameters    string   `json:"additionalParameters"    db:"additional_parameters"`
	MinimumSeeders          int      `json:"minimumSeeders"          db:"minimum_seeders"`      // Torznab only
	SeedRatio               *float64 `json:"seedRatio,omitempty"     db:"seed_ratio"`           // Torznab only, nil = use client default
	SeedTime                *int     `json:"seedTime,omitempty"      db:"seed_time"`            // Torznab only, minutes, nil = use client default
	CreatedAt               string   `json:"createdAt"               db:"created_at"`
	UpdatedAt               string   `json:"updatedAt"               db:"updated_at"`
}

// IndexerOptions holds global indexer settings.
type IndexerOptions struct {
	MinimumAge         int    `json:"minimumAge"         db:"minimum_age"`         // Minutes (Usenet: min age before grab)
	Retention          int    `json:"retention"          db:"retention"`           // Days (Usenet: 0 = unlimited)
	MaximumSize        int    `json:"maximumSize"        db:"maximum_size"`        // MB (0 = unlimited)
	RSSSyncInterval    int    `json:"rssSyncInterval"    db:"rss_sync_interval"`   // Minutes (0 = disabled)
	PreferIndexerFlags bool   `json:"preferIndexerFlags" db:"prefer_indexer_flags"` // Prioritize releases with special flags
	AvailabilityDelay  int    `json:"availabilityDelay"  db:"availability_delay"`  // Days
	UpdatedAt          string `json:"updatedAt"          db:"updated_at"`
}
