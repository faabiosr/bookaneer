package wanted

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/woliveiras/bookaneer/internal/download"
)

// GetDownloadQueue returns the current download queue.
func (s *Service) GetDownloadQueue(ctx context.Context) ([]DownloadQueueItem, error) {
	var items []DownloadQueueItem
	if err := s.db.SelectContext(ctx, &items, `
		SELECT dq.id, dq.book_id, dq.download_client_id, dq.indexer_id, dq.external_id,
		       dq.title, dq.size, dq.format, dq.status, dq.progress, dq.download_url, dq.added_at,
		       COALESCE(b.title, '') as book_title,
		       COALESCE(dc.name, '') as client_name
		FROM download_queue dq
		LEFT JOIN books b ON b.id = dq.book_id
		LEFT JOIN download_clients dc ON dc.id = dq.download_client_id
		ORDER BY dq.added_at DESC
	`); err != nil {
		return nil, err
	}

	// Apply fallbacks for NULL-joined columns.
	for i := range items {
		if items[i].BookTitle == "" {
			items[i].BookTitle = items[i].Title
		}
		if items[i].ClientName == "" {
			items[i].ClientName = "Embedded Downloader"
		}
	}

	// For embedded client items, get real-time status from the direct client
	client, _, err := s.downloadService.GetDirectClient(ctx)
	if err == nil && client != nil {
		for i := range items {
			// Check items with no client ID (embedded) that have active statuses
			if items[i].DownloadClientID == nil && items[i].ExternalID != "" {
				status, err := client.GetStatus(ctx, items[i].ExternalID)
				if err == nil {
					// Update with real-time status from embedded client
					items[i].Status = string(status.Status)
					items[i].Progress = status.Progress
					// Also update DB to persist the status
					_ = s.UpdateQueueItemStatus(ctx, items[i].ID, items[i].Status, items[i].Progress)
				}
			}
		}
	}

	return items, nil
}

// UpdateQueueItemStatus updates the status of a queue item.
func (s *Service) UpdateQueueItemStatus(ctx context.Context, id int64, status string, progress float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE download_queue SET status = ?, progress = ? WHERE id = ?`, status, progress, id)
	return err
}

// UpdateQueueItemStatusWithPath updates the status and save_path of a queue item.
func (s *Service) UpdateQueueItemStatusWithPath(ctx context.Context, id int64, status string, progress float64, savePath string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE download_queue SET status = ?, progress = ?, save_path = ? WHERE id = ?`, status, progress, savePath, id)
	return err
}

// RemoveFromQueue removes an item from the download queue.
func (s *Service) RemoveFromQueue(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM download_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete query failed: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("queue item %d not found", id)
	}
	return nil
}

// recordDownload adds an entry to the download_queue table.
// clientID can be nil for embedded client (no database entry).
func (s *Service) recordDownload(ctx context.Context, bookID int64, clientID *int64, indexerID *int64, title string, size int64, format, downloadURL, externalID, savePath string) error {
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO download_queue (book_id, download_client_id, indexer_id, external_id, title, size, format, status, download_url, save_path)
		VALUES (:book_id, :download_client_id, :indexer_id, :external_id, :title, :size, :format, 'queued', :download_url, :save_path)
	`, map[string]any{
		"book_id":            bookID,
		"download_client_id": clientID,
		"indexer_id":         indexerID,
		"external_id":        externalID,
		"title":              title,
		"size":               size,
		"format":             format,
		"download_url":       downloadURL,
		"save_path":          savePath,
	})
	return err
}

// restartDownload restarts a download that was lost (e.g., after server restart).
func (s *Service) restartDownload(ctx context.Context, queueID int64, client download.Client) error {
	// Get download info from queue
	var queueItem struct {
		Title       string `db:"title"`
		DownloadURL string `db:"download_url"`
	}
	if err := s.db.GetContext(ctx, &queueItem, `
		SELECT title, download_url FROM download_queue WHERE id = ?
	`, queueID); err != nil {
		return fmt.Errorf("get queue item: %w", err)
	}
	title, downloadURL := queueItem.Title, queueItem.DownloadURL

	if downloadURL == "" {
		return fmt.Errorf("no download URL for queue item %d", queueID)
	}

	// Add to client again
	newID, err := client.Add(ctx, download.AddItem{
		Name:        title,
		DownloadURL: downloadURL,
		Category:    "books",
	})
	if err != nil {
		return fmt.Errorf("add to client: %w", err)
	}

	// Update external_id in queue
	_, err = s.db.ExecContext(ctx, `UPDATE download_queue SET external_id = ?, status = 'queued' WHERE id = ?`, newID, queueID)
	if err != nil {
		return fmt.Errorf("update queue: %w", err)
	}

	slog.Info("Download restarted", "queueId", queueID, "newExternalId", newID)
	return nil
}
