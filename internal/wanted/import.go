package wanted

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/woliveiras/bookaneer/internal/download"
)

// ProcessDownloadsResult contains the results of processing downloads.
type ProcessDownloadsResult struct {
	Checked   int `json:"checked"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Imported  int `json:"imported"`
}

// ProcessDownloads checks active downloads and updates their status.
func (s *Service) ProcessDownloads(ctx context.Context) (*ProcessDownloadsResult, error) {
	result := &ProcessDownloadsResult{}

	// First, process any completed downloads that have a save_path but weren't imported
	// This handles server restarts where the in-memory download state was lost
	if imported, err := s.importPendingCompletedDownloads(ctx); err != nil {
		slog.Warn("Failed to import pending downloads", "error", err)
	} else {
		result.Imported = imported
	}

	// Get active downloads (queued, downloading, paused)
	type activeDownload struct {
		ID       int64  `db:"id"`
		ClientID *int64 `db:"download_client_id"`
		ExtID    string `db:"external_id"`
		Status   string `db:"status"`
	}

	var downloads []activeDownload
	if err := s.db.SelectContext(ctx, &downloads, `
		SELECT q.id, q.download_client_id, q.external_id, q.status
		FROM download_queue q
		WHERE q.status IN ('queued', 'downloading', 'paused', 'sent')
	`); err != nil {
		return nil, fmt.Errorf("query active downloads: %w", err)
	}

	result.Checked = len(downloads)

	// Check status of each download
	for _, d := range downloads {
		// Get appropriate client - use embedded client for NULL clientID
		client, _, err := s.downloadService.GetDirectClient(ctx)
		if err != nil || client == nil {
			slog.Warn("Could not get download client", "queueId", d.ID, "error", err)
			continue
		}

		status, err := client.GetStatus(ctx, d.ExtID)
		if err != nil {
			// Download not found in client - probably lost after restart
			// Try to restart the download
			slog.Info("Restarting lost download", "queueId", d.ID, "externalId", d.ExtID)
			if err := s.restartDownload(ctx, d.ID, client); err != nil {
				slog.Warn("Failed to restart download", "queueId", d.ID, "error", err)
			}
			continue
		}

		// Update status based on download client response (including save_path)
		newStatus := string(status.Status)
		if status.SavePath != "" {
			if err := s.UpdateQueueItemStatusWithPath(ctx, d.ID, newStatus, status.Progress, status.SavePath); err != nil {
				slog.Warn("Failed to update queue status", "id", d.ID, "error", err)
				continue
			}
		} else {
			if err := s.UpdateQueueItemStatus(ctx, d.ID, newStatus, status.Progress); err != nil {
				slog.Warn("Failed to update queue status", "id", d.ID, "error", err)
				continue
			}
		}

		switch status.Status {
		case download.StatusCompleted:
			result.Completed++
			// Import file to library
			if status.SavePath != "" {
				mismatch, err := s.importCompletedDownload(ctx, d.ID, status.SavePath)
				if err != nil {
					slog.Warn("Failed to import download",
						"queueId", d.ID,
						"path", status.SavePath,
						"error", err,
					)
				} else {
					slog.Info("Download imported to library",
						"queueId", d.ID,
						"path", status.SavePath,
						"contentMismatch", mismatch,
					)
					result.Imported++

					// If content mismatch detected and alternative sources exist, try next source
					if mismatch {
						slog.Warn("Content mismatch — trying next download source",
							"queueId", d.ID,
						)
						s.tryNextSourceForMismatch(ctx, d.ID)
					} else {
						// Clean up search results after successful import with verified content
						s.cleanupSearchResults(ctx, d.ID)
					}
				}
			}
		case download.StatusFailed:
			result.Failed++
			slog.Warn("Download failed",
				"queueId", d.ID,
				"error", status.ErrorMessage,
			)

			// Try next available source automatically
			if retried := s.tryNextSource(ctx, d.ID, status.ErrorMessage); retried {
				slog.Info("Automatically trying next download source", "queueId", d.ID)
			}
		}
	}

	return result, nil
}

// importPendingCompletedDownloads imports downloads that completed but weren't imported
// (e.g., because the server restarted before import could happen).
func (s *Service) importPendingCompletedDownloads(ctx context.Context) (int, error) {
	// Find completed downloads with save_path that haven't been imported yet
	// (not imported = no entry in book_files for that book_id)
	// Collect all pending imports first, then process after rows are closed.
	// This avoids SQLite lock issues when doing writes during iteration.
	type pendingImport struct {
		QueueID  int64  `db:"id"`
		BookID   int64  `db:"book_id"`
		SavePath string `db:"save_path"`
	}
	var pending []pendingImport
	if err := s.db.SelectContext(ctx, &pending, `
		SELECT q.id, q.book_id, q.save_path
		FROM download_queue q
		WHERE q.status = 'completed'
		  AND q.save_path != ''
		  AND NOT EXISTS (SELECT 1 FROM book_files bf WHERE bf.book_id = q.book_id)
	`); err != nil {
		return 0, fmt.Errorf("query pending imports: %w", err)
	}

	var imported int
	for _, p := range pending {
		// Apply remote path mapping before checking file existence
		localPath := p.SavePath
		if s.pathMapper != nil {
			localPath = s.pathMapper.MapPath(ctx, localPath)
		}

		// Check if file still exists
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			slog.Warn("Download file no longer exists, marking as failed",
				"queueId", p.QueueID,
				"path", localPath,
			)
			_ = s.UpdateQueueItemStatus(ctx, p.QueueID, "failed", 0)
			continue
		}

		// Import the download
		if _, err := s.importCompletedDownload(ctx, p.QueueID, p.SavePath); err != nil {
			slog.Warn("Failed to import pending download",
				"queueId", p.QueueID,
				"path", p.SavePath,
				"error", err,
			)
		} else {
			slog.Info("Successfully imported pending download",
				"queueId", p.QueueID,
				"path", p.SavePath,
			)
			imported++
		}
	}

	return imported, nil
}
