package wanted

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/woliveiras/bookaneer/internal/core/book"
	"github.com/woliveiras/bookaneer/internal/library"
	"github.com/woliveiras/bookaneer/internal/search"
)

// Search result status values stored in the search_results table.
const (
	searchResultPending = "pending"
	searchResultTried   = "tried"
	searchResultFailed  = "failed"
	searchResultSuccess = "success"
)

// searchDigitalLibraries searches digital libraries and saves all results for fallback.
func (s *Service) searchDigitalLibraries(ctx context.Context, b *book.Book, query string) (*GrabResult, error) {
	if s.libraryService == nil {
		return nil, nil // Not configured
	}

	results, err := s.libraryService.Search(ctx, b.Title) // Use just title for library search
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Clear any previous search results for this book
	if _, err := s.db.ExecContext(ctx, `DELETE FROM search_results WHERE book_id = ?`, b.ID); err != nil {
		slog.Warn("failed to clear search results", "bookId", b.ID, "error", err)
	}

	// Filter and save all valid results for fallback
	var validResults []library.SearchResult
	priority := 0
	for _, r := range results {
		// Verify it's a downloadable format
		format := strings.ToLower(r.Format)
		if format != "epub" && format != "pdf" && format != "mobi" {
			continue
		}

		// Need a download URL
		if r.DownloadURL == "" {
			continue
		}

		validResults = append(validResults, r)

		// Save to search_results table for fallback
		_, err := s.db.NamedExecContext(ctx, `
			INSERT INTO search_results (book_id, provider, title, download_url, format, size, score, priority, status)
			VALUES (:book_id, :provider, :title, :download_url, :format, :size, :score, :priority, :status)
		`, map[string]any{
			"book_id":      b.ID,
			"provider":     r.Provider,
			"title":        r.Title,
			"download_url": r.DownloadURL,
			"format":       r.Format,
			"size":         r.Size,
			"score":        r.Score,
			"priority":     priority,
			"status":       searchResultPending,
		})
		if err != nil {
			slog.Warn("Failed to save search result", "error", err)
		}
		priority++
	}

	if len(validResults) == 0 {
		return nil, nil
	}

	slog.Info("Found download sources", "book", b.Title, "count", len(validResults))

	// Try to grab the first (best) result
	return s.grabNextSearchResult(ctx, b)
}

// grabNextSearchResult tries to grab the next pending search result for a book.
func (s *Service) grabNextSearchResult(ctx context.Context, b *book.Book) (*GrabResult, error) {
	// Get next pending result
	var nextResult struct {
		ID          int64  `db:"id"`
		Provider    string `db:"provider"`
		Title       string `db:"title"`
		DownloadURL string `db:"download_url"`
		Format      string `db:"format"`
		Size        int64  `db:"size"`
	}
	if err := s.db.GetContext(ctx, &nextResult, `
		SELECT id, provider, title, download_url, format, size
		FROM search_results
		WHERE book_id = ? AND status = ?
		ORDER BY priority ASC
		LIMIT 1
	`, b.ID, searchResultPending); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no more download sources available for %q", b.Title)
		}
		return nil, fmt.Errorf("get next search result: %w", err)
	}
	resultID := nextResult.ID

	// Mark as tried
	if _, err := s.db.ExecContext(ctx, `
		UPDATE search_results SET status = ?, tried_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ?
	`, searchResultTried, resultID); err != nil {
		slog.Warn("failed to mark search result as tried", "resultId", resultID, "error", err)
	}

	// Build library result struct
	r := &library.SearchResult{
		Provider:    nextResult.Provider,
		Title:       nextResult.Title,
		DownloadURL: nextResult.DownloadURL,
		Format:      nextResult.Format,
		Size:        nextResult.Size,
	}

	// Try to grab it
	grabResult, err := s.grabFromLibrary(ctx, b, r)
	if err != nil {
		// Mark as failed and record error
		if _, ferr := s.db.ExecContext(ctx, `
			UPDATE search_results SET status = ?, error_message = ? WHERE id = ?
		`, searchResultFailed, err.Error(), resultID); ferr != nil {
			slog.Warn("failed to mark search result as failed", "resultId", resultID, "error", ferr)
		}
		return nil, err
	}

	// Mark as success
	if _, err := s.db.ExecContext(ctx, `UPDATE search_results SET status = ? WHERE id = ?`, searchResultSuccess, resultID); err != nil {
		slog.Warn("failed to mark search result as success", "resultId", resultID, "error", err)
	}

	return grabResult, nil
}

// searchIndexers searches torrent/usenet indexers and grabs the best result.
func (s *Service) searchIndexers(ctx context.Context, b *book.Book, query string) (*GrabResult, error) {
	if s.searchService == nil {
		return nil, nil
	}

	results, err := s.searchService.Search(ctx, search.SearchQuery{Query: query})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Filter for ebook formats
	var filtered []search.Result
	for _, r := range results {
		if isEbookFormat(r.Title) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Try to grab the first suitable result
	for _, r := range filtered {
		grabResult, err := s.grabFromIndexer(ctx, b, &r)
		if err != nil {
			slog.Warn("Failed to grab from indexer", "indexer", r.IndexerName, "error", err)
			continue
		}
		return grabResult, nil
	}

	return nil, nil
}

// tryNextSource attempts to download from the next available source when a download fails.
// Returns true if a new download was started, false if no more sources are available.
func (s *Service) tryNextSource(ctx context.Context, queueID int64, errorMessage string) bool {
	// Get book_id from queue
	var bookID int64
	err := s.db.GetContext(ctx, &bookID, `SELECT book_id FROM download_queue WHERE id = ?`, queueID)
	if err != nil {
		slog.Warn("Failed to get book_id for retry", "queueId", queueID, "error", err)
		return false
	}

	// Get book info
	b, err := s.bookService.FindByID(ctx, bookID)
	if err != nil {
		slog.Warn("Failed to get book for retry", "bookId", bookID, "error", err)
		return false
	}

	// Check if there are more sources to try
	var pendingCount int
	err = s.db.GetContext(ctx, &pendingCount, `
		SELECT COUNT(*) FROM search_results WHERE book_id = ? AND status = ?
	`, bookID, searchResultPending)
	if err != nil || pendingCount == 0 {
		slog.Info("No more download sources available", "book", b.Title)
		return false
	}

	// Remove the failed queue entry
	if _, err := s.db.ExecContext(ctx, `DELETE FROM download_queue WHERE id = ?`, queueID); err != nil {
		slog.Warn("failed to remove failed queue entry", "queueId", queueID, "error", err)
	}

	// Try next source
	grabResult, err := s.grabNextSearchResult(ctx, b)
	if err != nil {
		slog.Warn("Failed to grab next source", "book", b.Title, "error", err)
		return false
	}

	slog.Info("Retrying with alternative source",
		"book", b.Title,
		"source", grabResult.ProviderName,
		"remaining", pendingCount-1,
	)

	return true
}

// cleanupSearchResults removes search results after successful download.
func (s *Service) cleanupSearchResults(ctx context.Context, queueID int64) {
	// Get book_id from queue
	var bookID int64
	err := s.db.GetContext(ctx, &bookID, `SELECT book_id FROM download_queue WHERE id = ?`, queueID)
	if err != nil {
		return
	}

	// Delete all search results for this book
	if _, err := s.db.ExecContext(ctx, `DELETE FROM search_results WHERE book_id = ?`, bookID); err != nil {
		slog.Warn("failed to cleanup search results", "bookId", bookID, "error", err)
	}
}

// GetPendingSourcesCount returns the number of pending download sources for a book.
func (s *Service) GetPendingSourcesCount(ctx context.Context, bookID int64) int {
	var count int
	_ = s.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM search_results WHERE book_id = ? AND status = ?
	`, bookID, searchResultPending)
	return count
}

// tryNextSourceForMismatch handles content mismatch by blocklisting the bad source
// and attempting the next available one. The mismatched file is kept (flagged)
// so the user can inspect it.
func (s *Service) tryNextSourceForMismatch(ctx context.Context, queueID int64) {
	var queueItem struct {
		BookID      int64  `db:"book_id"`
		Title       string `db:"title"`
		DownloadURL string `db:"download_url"`
		Format      string `db:"format"`
	}
	if err := s.db.GetContext(ctx, &queueItem, `
		SELECT book_id, title, download_url, format FROM download_queue WHERE id = ?
	`, queueID); err != nil {
		slog.Warn("failed to get queue item for mismatch retry", "queueId", queueID, "error", err)
		return
	}
	bookID, title, format := queueItem.BookID, queueItem.Title, queueItem.Format

	// Blocklist the bad source so we don't try it again
	_ = s.AddToBlocklist(ctx, bookID, title, format, "content mismatch detected automatically")

	// Check if there are more sources to try
	pending := s.GetPendingSourcesCount(ctx, bookID)
	if pending == 0 {
		slog.Info("No alternative sources for content mismatch", "book", title)
		return
	}

	b, err := s.bookService.FindByID(ctx, bookID)
	if err != nil {
		return
	}

	grabResult, err := s.grabNextSearchResult(ctx, b)
	if err != nil {
		slog.Warn("Failed to grab next source after mismatch", "book", b.Title, "error", err)
		return
	}

	slog.Info("Retrying with alternative source after content mismatch",
		"book", b.Title,
		"source", grabResult.ProviderName,
		"remaining", pending-1,
	)
}
