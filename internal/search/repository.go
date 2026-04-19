package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ListIndexers returns all indexers.
func (s *Service) ListIndexers(ctx context.Context) ([]IndexerConfig, error) {
	var indexers []IndexerConfig
	if err := s.db.SelectContext(ctx, &indexers, `
		SELECT id, name, type, base_url, api_path, api_key, categories, priority, enabled,
		       enable_rss, enable_automatic_search, enable_interactive_search,
		       additional_parameters, minimum_seeders, seed_ratio, seed_time,
		       created_at, updated_at
		FROM indexers ORDER BY priority ASC
	`); err != nil {
		return nil, fmt.Errorf("query indexers: %w", err)
	}
	return indexers, nil
}

// GetIndexer returns an indexer by ID.
func (s *Service) GetIndexer(ctx context.Context, id int64) (*IndexerConfig, error) {
	var cfg IndexerConfig
	err := s.db.GetContext(ctx, &cfg, `
		SELECT id, name, type, base_url, api_path, api_key, categories, priority, enabled,
		       enable_rss, enable_automatic_search, enable_interactive_search,
		       additional_parameters, minimum_seeders, seed_ratio, seed_time,
		       created_at, updated_at
		FROM indexers WHERE id = ?
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIndexerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query indexer: %w", err)
	}
	return &cfg, nil
}

// CreateIndexer creates a new indexer.
func (s *Service) CreateIndexer(ctx context.Context, cfg IndexerConfig) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.NamedExecContext(ctx, `
		INSERT INTO indexers (name, type, base_url, api_path, api_key, categories, priority, enabled,
		    enable_rss, enable_automatic_search, enable_interactive_search,
		    additional_parameters, minimum_seeders, seed_ratio, seed_time,
		    created_at, updated_at)
		VALUES (:name, :type, :base_url, :api_path, :api_key, :categories, :priority, :enabled,
		    :enable_rss, :enable_automatic_search, :enable_interactive_search,
		    :additional_parameters, :minimum_seeders, :seed_ratio, :seed_time,
		    :created_at, :updated_at)
	`, map[string]any{
		"name":                      cfg.Name,
		"type":                      cfg.Type,
		"base_url":                  cfg.BaseURL,
		"api_path":                  cfg.APIPath,
		"api_key":                   cfg.APIKey,
		"categories":                cfg.Categories,
		"priority":                  cfg.Priority,
		"enabled":                   cfg.Enabled,
		"enable_rss":                cfg.EnableRSS,
		"enable_automatic_search":   cfg.EnableAutomaticSearch,
		"enable_interactive_search": cfg.EnableInteractiveSearch,
		"additional_parameters":     cfg.AdditionalParameters,
		"minimum_seeders":           cfg.MinimumSeeders,
		"seed_ratio":                cfg.SeedRatio,
		"seed_time":                 cfg.SeedTime,
		"created_at":                now,
		"updated_at":                now,
	})
	if err != nil {
		return 0, fmt.Errorf("insert indexer: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}
	cfg.ID = id
	if cfg.Enabled {
		s.loadClient(cfg)
	}
	return id, nil
}

// UpdateIndexer updates an existing indexer.
func (s *Service) UpdateIndexer(ctx context.Context, cfg IndexerConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.NamedExecContext(ctx, `
		UPDATE indexers SET name = :name, type = :type, base_url = :base_url, api_path = :api_path, api_key = :api_key,
		categories = :categories, priority = :priority, enabled = :enabled,
		enable_rss = :enable_rss, enable_automatic_search = :enable_automatic_search,
		enable_interactive_search = :enable_interactive_search,
		additional_parameters = :additional_parameters, minimum_seeders = :minimum_seeders,
		seed_ratio = :seed_ratio, seed_time = :seed_time,
		updated_at = :updated_at WHERE id = :id
	`, map[string]any{
		"name":                      cfg.Name,
		"type":                      cfg.Type,
		"base_url":                  cfg.BaseURL,
		"api_path":                  cfg.APIPath,
		"api_key":                   cfg.APIKey,
		"categories":                cfg.Categories,
		"priority":                  cfg.Priority,
		"enabled":                   cfg.Enabled,
		"enable_rss":                cfg.EnableRSS,
		"enable_automatic_search":   cfg.EnableAutomaticSearch,
		"enable_interactive_search": cfg.EnableInteractiveSearch,
		"additional_parameters":     cfg.AdditionalParameters,
		"minimum_seeders":           cfg.MinimumSeeders,
		"seed_ratio":                cfg.SeedRatio,
		"seed_time":                 cfg.SeedTime,
		"updated_at":                now,
		"id":                        cfg.ID,
	})
	if err != nil {
		return fmt.Errorf("update indexer: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrIndexerNotFound
	}
	s.mu.Lock()
	delete(s.clients, cfg.ID)
	s.mu.Unlock()
	if cfg.Enabled {
		s.loadClient(cfg)
	}
	return nil
}

// DeleteIndexer deletes an indexer.
func (s *Service) DeleteIndexer(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM indexers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete indexer: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrIndexerNotFound
	}
	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()
	return nil
}

// GetOptions returns the global indexer options.
func (s *Service) GetOptions(ctx context.Context) (*IndexerOptions, error) {
	var opts IndexerOptions
	err := s.db.GetContext(ctx, &opts, `
		SELECT minimum_age, retention, maximum_size, rss_sync_interval,
		       prefer_indexer_flags, availability_delay, updated_at
		FROM indexer_options WHERE id = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("query indexer options: %w", err)
	}
	return &opts, nil
}

// UpdateOptions updates the global indexer options.
func (s *Service) UpdateOptions(ctx context.Context, opts IndexerOptions) error {
	now := time.Now().UTC().Format(time.RFC3339)
	preferFlags := 0
	if opts.PreferIndexerFlags {
		preferFlags = 1
	}
	_, err := s.db.NamedExecContext(ctx, `
		UPDATE indexer_options SET
		    minimum_age = :minimum_age, retention = :retention, maximum_size = :maximum_size,
		    rss_sync_interval = :rss_sync_interval, prefer_indexer_flags = :prefer_indexer_flags,
		    availability_delay = :availability_delay, updated_at = :updated_at
		WHERE id = 1
	`, map[string]any{
		"minimum_age":         opts.MinimumAge,
		"retention":           opts.Retention,
		"maximum_size":        opts.MaximumSize,
		"rss_sync_interval":   opts.RSSSyncInterval,
		"prefer_indexer_flags": preferFlags,
		"availability_delay":  opts.AvailabilityDelay,
		"updated_at":          now,
	})
	if err != nil {
		return fmt.Errorf("update indexer options: %w", err)
	}
	return nil
}
