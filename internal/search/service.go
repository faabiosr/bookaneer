package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
)

// Service manages indexers and search operations.
type Service struct {
	db      *sqlx.DB
	clients map[int64]Indexer
	mu      sync.RWMutex
}

// NewService creates a new search service.
func NewService(db *sqlx.DB) *Service {
	return &Service{
		db:      db,
		clients: make(map[int64]Indexer),
	}
}

// LoadIndexers loads all enabled indexers from the database.
func (s *Service) LoadIndexers(ctx context.Context) error {
	var indexers []IndexerConfig
	if err := s.db.SelectContext(ctx, &indexers, `
		SELECT id, name, type, base_url, api_path, api_key, categories, priority, enabled,
		       enable_rss, enable_automatic_search, enable_interactive_search,
		       additional_parameters, minimum_seeders, seed_ratio, seed_time,
		       created_at, updated_at
		FROM indexers WHERE enabled = 1
	`); err != nil {
		return fmt.Errorf("query indexers: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = make(map[int64]Indexer)

	for _, cfg := range indexers {
		factory, ok := GetFactory(cfg.Type)
		if !ok {
			continue
		}
		indexer, err := factory(cfg)
		if err != nil {
			continue
		}
		s.clients[cfg.ID] = indexer
	}
	return nil
}

// TestIndexer tests the connection to an indexer.
func (s *Service) TestIndexer(ctx context.Context, cfg IndexerConfig) error {
	factory, ok := GetFactory(cfg.Type)
	if !ok {
		return fmt.Errorf("unknown indexer type: %s", cfg.Type)
	}
	indexer, err := factory(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}
	return indexer.Test(ctx)
}

// Search searches all enabled indexers concurrently.
func (s *Service) Search(ctx context.Context, query SearchQuery) ([]Result, error) {
	s.mu.RLock()
	clients := make([]Indexer, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	if len(clients) == 0 {
		return nil, nil
	}

	type indexerResult struct {
		results []Result
		err     error
	}
	resultCh := make(chan indexerResult, len(clients))

	for _, c := range clients {
		go func(indexer Indexer) {
			results, err := indexer.Search(ctx, query)
			resultCh <- indexerResult{results, err}
		}(c)
	}

	var allResults []Result
	var errs []string
	for range clients {
		r := <-resultCh
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		allResults = append(allResults, r.results...)
	}
	if len(allResults) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all indexers failed: %s", strings.Join(errs, "; "))
	}
	return allResults, nil
}

func (s *Service) loadClient(cfg IndexerConfig) {
	factory, ok := GetFactory(cfg.Type)
	if !ok {
		return
	}
	indexer, err := factory(cfg)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.clients[cfg.ID] = indexer
	s.mu.Unlock()
}
