// Package wanted provides services for searching and grabbing wanted books.
// It coordinates between metadata providers, digital libraries, indexers, and download clients.
package wanted

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/woliveiras/bookaneer/internal/core/book"
	corelibrary "github.com/woliveiras/bookaneer/internal/core/library"
	"github.com/woliveiras/bookaneer/internal/core/naming"
	"github.com/woliveiras/bookaneer/internal/core/pathmapping"
	"github.com/woliveiras/bookaneer/internal/download"
	"github.com/woliveiras/bookaneer/internal/library"
	"github.com/woliveiras/bookaneer/internal/search"
)

// Service handles searching and grabbing wanted books.
type Service struct {
	db              *sqlx.DB
	bookService     *book.Service
	libraryService  *library.Aggregator
	searchService   *search.Service
	downloadService *download.Service
	namingEngine    *naming.Engine
	scanner         *corelibrary.Scanner
	pathMapper      *pathmapping.Service
}

// New creates a new Wanted service.
func New(
	db *sqlx.DB,
	bookService *book.Service,
	libraryService *library.Aggregator,
	searchService *search.Service,
	downloadService *download.Service,
	namingEngine *naming.Engine,
	scanner *corelibrary.Scanner,
	pathMapper *pathmapping.Service,
) *Service {
	return &Service{
		db:              db,
		bookService:     bookService,
		libraryService:  libraryService,
		searchService:   searchService,
		downloadService: downloadService,
		namingEngine:    namingEngine,
		scanner:         scanner,
		pathMapper:      pathMapper,
	}
}

// GetBookInfo returns book title and author for display purposes.
func (s *Service) GetBookInfo(ctx context.Context, bookID int64) (title, authorName string, err error) {
	b, err := s.bookService.FindByID(ctx, bookID)
	if err != nil {
		return "", "", err
	}
	return b.Title, b.AuthorName, nil
}

// SearchAndGrab searches for a book and grabs the best result.
func (s *Service) SearchAndGrab(ctx context.Context, bookID int64) (*GrabResult, error) {
	// Get book details
	b, err := s.bookService.FindByID(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("find book: %w", err)
	}

	if !b.Monitored {
		return nil, fmt.Errorf("book %d is not monitored", bookID)
	}

	// Check if there's already an active download for this book
	var activeCount int
	err = s.db.GetContext(ctx, &activeCount, `
		SELECT COUNT(*) FROM download_queue
		WHERE book_id = ? AND status IN ('queued', 'downloading', 'paused', 'importing')
	`, bookID)
	if err != nil {
		slog.Warn("Failed to check for active downloads", "error", err)
	} else if activeCount > 0 {
		slog.Info("Skipping search - book already has active download", "id", bookID, "title", b.Title)
		return nil, fmt.Errorf("book already has an active download in queue")
	}

	slog.Info("Searching for book", "id", bookID, "title", b.Title)

	// Build search query
	query := b.Title
	if b.AuthorName != "" {
		query = fmt.Sprintf("%s %s", b.Title, b.AuthorName)
	}

	// Try digital libraries first (free, direct download)
	result, err := s.searchDigitalLibraries(ctx, b, query)
	if err == nil && result != nil {
		return result, nil
	}
	if err != nil {
		slog.Warn("Digital library search failed", "error", err)
	}

	// Fall back to indexers (torrent/usenet)
	result, err = s.searchIndexers(ctx, b, query)
	if err == nil && result != nil {
		return result, nil
	}
	if err != nil {
		slog.Warn("Indexer search failed", "error", err)
	}

	return nil, fmt.Errorf("no suitable download found for %q", b.Title)
}

// GetWantedBooks returns all monitored books without files.
func (s *Service) GetWantedBooks(ctx context.Context) ([]book.Book, error) {
	var books []book.Book
	err := s.db.SelectContext(ctx, &books, `
		SELECT b.id, b.author_id, b.title,
		       COALESCE(b.sort_title,'')   as sort_title,
		       COALESCE(b.foreign_id,'')   as foreign_id,
		       COALESCE(b.isbn,'')         as isbn,
		       COALESCE(b.isbn13,'')       as isbn13,
		       COALESCE(b.release_date,'') as release_date,
		       COALESCE(b.overview,'')     as overview,
		       COALESCE(b.image_url,'')    as image_url,
		       b.page_count, b.monitored,
		       b.added_at, b.updated_at,
		       COALESCE(a.name,'') as name,
		       EXISTS(SELECT 1 FROM book_files bf WHERE bf.book_id = b.id) as has_file
		FROM books b
		LEFT JOIN authors a ON a.id = b.author_id
		WHERE b.monitored = 1
		  AND NOT EXISTS (SELECT 1 FROM book_files bf WHERE bf.book_id = b.id)
		ORDER BY b.added_at DESC
	`)
	return books, err
}

// SearchAllWanted searches for all wanted books.
func (s *Service) SearchAllWanted(ctx context.Context) ([]GrabResult, error) {
	books, err := s.GetWantedBooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("get wanted books: %w", err)
	}

	slog.Info("Searching for wanted books", "count", len(books))

	var results []GrabResult
	for _, b := range books {
		result, err := s.SearchAndGrab(ctx, b.ID)
		if err != nil {
			slog.Warn("Failed to grab book", "id", b.ID, "title", b.Title, "error", err)
			continue
		}
		if result != nil {
			results = append(results, *result)
		}

		// Small delay between searches to be nice to providers
		time.Sleep(500 * time.Millisecond)
	}

	return results, nil
}
