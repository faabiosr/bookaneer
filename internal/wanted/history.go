package wanted

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// GetHistory returns recent history events.
func (s *Service) GetHistory(ctx context.Context, limit int, eventType string) ([]HistoryItem, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT h.id, h.book_id, h.author_id, h.event_type, h.source_title, h.quality, h.data, h.date,
		       COALESCE(b.title, '') as book_title,
		       COALESCE(a.name, '') as author_name
		FROM history h
		LEFT JOIN books b ON b.id = h.book_id
		LEFT JOIN authors a ON a.id = h.author_id
	`
	var args []any
	if eventType != "" {
		query += " WHERE h.event_type = ?"
		args = append(args, eventType)
	}
	query += " ORDER BY h.date DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		var bookID, authorID sql.NullInt64
		var dataJSON string
		if err := rows.Scan(&item.ID, &bookID, &authorID, &item.EventType, &item.SourceTitle, &item.Quality, &dataJSON, &item.Date, &item.BookTitle, &item.AuthorName); err != nil {
			return nil, err
		}
		if bookID.Valid {
			item.BookID = &bookID.Int64
		}
		if authorID.Valid {
			item.AuthorID = &authorID.Int64
		}
		_ = json.Unmarshal([]byte(dataJSON), &item.Data)
		items = append(items, item)
	}

	return items, rows.Err()
}

// recordHistory adds an entry to the history table.
func (s *Service) recordHistory(ctx context.Context, bookID, authorID int64, eventType, sourceTitle, quality string, data map[string]any) {
	dataJSON, _ := json.Marshal(data)
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO history (book_id, author_id, event_type, source_title, quality, data)
		VALUES (:book_id, :author_id, :event_type, :source_title, :quality, :data)
	`, map[string]any{
		"book_id":      bookID,
		"author_id":    authorID,
		"event_type":   eventType,
		"source_title": sourceTitle,
		"quality":      quality,
		"data":         string(dataJSON),
	})
	if err != nil {
		slog.Error("Failed to record history", "error", err)
	}
}

// GetBlocklist returns all blocklisted releases.
func (s *Service) GetBlocklist(ctx context.Context) ([]BlocklistItem, error) {
	var items []BlocklistItem
	err := s.db.SelectContext(ctx, &items, `
		SELECT bl.id, bl.book_id, bl.source_title, bl.quality, bl.reason, bl.date,
		       COALESCE(b.title, '') as book_title,
		       COALESCE(a.name, '') as author_name
		FROM blocklist bl
		LEFT JOIN books b ON b.id = bl.book_id
		LEFT JOIN authors a ON a.id = b.author_id
		ORDER BY bl.date DESC
	`)
	return items, err
}

// AddToBlocklist adds a release to the blocklist.
func (s *Service) AddToBlocklist(ctx context.Context, bookID int64, sourceTitle, quality, reason string) error {
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO blocklist (book_id, source_title, quality, reason)
		VALUES (:book_id, :source_title, :quality, :reason)
	`, map[string]any{
		"book_id":      bookID,
		"source_title": sourceTitle,
		"quality":      quality,
		"reason":       reason,
	})
	return err
}

// RemoveFromBlocklist removes an item from the blocklist.
func (s *Service) RemoveFromBlocklist(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM blocklist WHERE id = ?`, id)
	return err
}

// ReportWrongContent handles when a user reports that a downloaded file has wrong content.
// It removes the book file, blocklists the source, and tries the next available source.
func (s *Service) ReportWrongContent(ctx context.Context, bookID int64) error {
	// Get and remove the book file
	var bookFile struct {
		ID     int64  `db:"id"`
		Path   string `db:"path"`
		Format string `db:"format"`
	}
	if err := s.db.GetContext(ctx, &bookFile, `
		SELECT id, path, format FROM book_files WHERE book_id = ?
	`, bookID); err != nil {
		return fmt.Errorf("no book file found for book %d: %w", bookID, err)
	}
	fileID, filePath, format := bookFile.ID, bookFile.Path, bookFile.Format

	// Get book info for history
	b, err := s.bookService.FindByID(ctx, bookID)
	if err != nil {
		return fmt.Errorf("find book: %w", err)
	}

	// Find the download URL from queue or history for blocklisting
	var sourceTitle string
	err = s.db.GetContext(ctx, &sourceTitle, `
		SELECT title FROM download_queue WHERE book_id = ? ORDER BY added_at DESC LIMIT 1
	`, bookID)
	if err != nil {
		sourceTitle = filePath // fallback to file path
	}

	// Delete the file from disk
	_ = os.Remove(filePath)

	// Remove from book_files
	_, _ = s.db.ExecContext(ctx, `DELETE FROM book_files WHERE id = ?`, fileID)

	// Blocklist this source
	_ = s.AddToBlocklist(ctx, bookID, sourceTitle, format, "wrong content reported by user")

	// Record history
	s.recordHistory(ctx, bookID, b.AuthorID, "wrongContent", b.Title, format, map[string]any{
		"path":        filePath,
		"sourceTitle": sourceTitle,
	})

	// Try next available source
	pending := s.GetPendingSourcesCount(ctx, bookID)
	if pending > 0 {
		grabResult, grabErr := s.grabNextSearchResult(ctx, b)
		if grabErr != nil {
			slog.Warn("Failed to grab next source after wrong content report", "book", b.Title, "error", grabErr)
		} else {
			slog.Info("Retrying download after wrong content report",
				"book", b.Title,
				"source", grabResult.ProviderName,
				"remaining", pending-1,
			)
		}
	}

	return nil
}
