package download

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const clientSelectColumns = `
	SELECT id, name, type, host, port, use_tls,
	       COALESCE(username, '') as username, COALESCE(password, '') as password,
	       COALESCE(api_key, '') as api_key, COALESCE(category, '') as category,
	       recent_priority, older_priority, remove_completed_after,
	       enabled, priority,
	       COALESCE(nzb_folder, '') as nzb_folder, COALESCE(torrent_folder, '') as torrent_folder,
	       COALESCE(watch_folder, '') as watch_folder, COALESCE(download_dir, '') as download_dir,
	       created_at, updated_at
	FROM download_clients
`

// ListClients returns all download clients from the database.
func (s *Service) ListClients(ctx context.Context) ([]ClientConfig, error) {
	var clients []ClientConfig
	if err := s.db.SelectContext(ctx, &clients, clientSelectColumns+`ORDER BY priority ASC, name ASC`); err != nil {
		return nil, fmt.Errorf("query clients: %w", err)
	}
	return clients, nil
}

// GetClient returns a download client by ID.
func (s *Service) GetClient(ctx context.Context, id int64) (*ClientConfig, error) {
	var cfg ClientConfig
	err := s.db.GetContext(ctx, &cfg, clientSelectColumns+`WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query client: %w", err)
	}
	return &cfg, nil
}

// CreateClient creates a new download client.
func (s *Service) CreateClient(ctx context.Context, cfg *ClientConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.NamedExecContext(ctx, `
		INSERT INTO download_clients (
			name, type, host, port, use_tls, username, password, api_key,
			category, recent_priority, older_priority, remove_completed_after,
			enabled, priority, nzb_folder, torrent_folder, watch_folder, download_dir,
			created_at, updated_at
		) VALUES (
			:name, :type, :host, :port, :use_tls, :username, :password, :api_key,
			:category, :recent_priority, :older_priority, :remove_completed_after,
			:enabled, :priority, :nzb_folder, :torrent_folder, :watch_folder, :download_dir,
			:created_at, :updated_at
		)
	`, map[string]any{
		"name":                   cfg.Name,
		"type":                   cfg.Type,
		"host":                   cfg.Host,
		"port":                   cfg.Port,
		"use_tls":                cfg.UseTLS,
		"username":               cfg.Username,
		"password":               cfg.Password,
		"api_key":                cfg.APIKey,
		"category":               cfg.Category,
		"recent_priority":        cfg.RecentPriority,
		"older_priority":         cfg.OlderPriority,
		"remove_completed_after": cfg.RemoveCompletedAfter,
		"enabled":                cfg.Enabled,
		"priority":               cfg.Priority,
		"nzb_folder":             cfg.NzbFolder,
		"torrent_folder":         cfg.TorrentFolder,
		"watch_folder":           cfg.WatchFolder,
		"download_dir":           cfg.DownloadDir,
		"created_at":             now,
		"updated_at":             now,
	})
	if err != nil {
		return fmt.Errorf("insert client: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	cfg.ID = id
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	return nil
}

// UpdateClient updates an existing download client.
func (s *Service) UpdateClient(ctx context.Context, cfg *ClientConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.NamedExecContext(ctx, `
		UPDATE download_clients SET
			name = :name, type = :type, host = :host, port = :port, use_tls = :use_tls,
			username = :username, password = :password, api_key = :api_key, category = :category,
			recent_priority = :recent_priority, older_priority = :older_priority,
			remove_completed_after = :remove_completed_after,
			enabled = :enabled, priority = :priority, nzb_folder = :nzb_folder,
			torrent_folder = :torrent_folder, watch_folder = :watch_folder,
			download_dir = :download_dir, updated_at = :updated_at
		WHERE id = :id
	`, map[string]any{
		"name":                   cfg.Name,
		"type":                   cfg.Type,
		"host":                   cfg.Host,
		"port":                   cfg.Port,
		"use_tls":                cfg.UseTLS,
		"username":               cfg.Username,
		"password":               cfg.Password,
		"api_key":                cfg.APIKey,
		"category":               cfg.Category,
		"recent_priority":        cfg.RecentPriority,
		"older_priority":         cfg.OlderPriority,
		"remove_completed_after": cfg.RemoveCompletedAfter,
		"enabled":                cfg.Enabled,
		"priority":               cfg.Priority,
		"nzb_folder":             cfg.NzbFolder,
		"torrent_folder":         cfg.TorrentFolder,
		"watch_folder":           cfg.WatchFolder,
		"download_dir":           cfg.DownloadDir,
		"updated_at":             now,
		"id":                     cfg.ID,
	})
	if err != nil {
		return fmt.Errorf("update client: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	cfg.UpdatedAt = now

	// Invalidate cached client
	s.mu.Lock()
	delete(s.clients, cfg.ID)
	s.mu.Unlock()

	return nil
}

// DeleteClient deletes a download client.
func (s *Service) DeleteClient(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM download_clients WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete client: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()

	return nil
}
