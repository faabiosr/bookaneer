package series

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// GetWithBooks returns a series with its books.
func (s *Service) GetWithBooks(ctx context.Context, id int64) (*SeriesWithBooks, error) {
	ser, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := &SeriesWithBooks{
		Series: *ser,
		Books:  []SeriesBook{},
	}

	if err := s.db.SelectContext(ctx, &result.Books, `
		SELECT sb.series_id, sb.book_id, sb.position, b.title AS book_title, a.name AS author_name
		FROM series_books sb
		JOIN books b ON sb.book_id = b.id
		JOIN authors a ON b.author_id = a.id
		WHERE sb.series_id = ?
		ORDER BY sb.position
	`, id); err != nil {
		return nil, fmt.Errorf("get series books: %w", err)
	}

	return result, nil
}

// AddBook adds a book to a series.
func (s *Service) AddBook(ctx context.Context, seriesID int64, input AddBookInput) error {
	_, err := s.FindByID(ctx, seriesID)
	if err != nil {
		return err
	}

	var bookExists int
	err = s.db.GetContext(ctx, &bookExists, "SELECT 1 FROM books WHERE id = ?", input.BookID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrBookNotFound
		}
		return fmt.Errorf("check book: %w", err)
	}

	_, err = s.db.NamedExecContext(ctx, `
		INSERT INTO series_books (series_id, book_id, position)
		VALUES (:series_id, :book_id, :position)
	`, map[string]any{
		"series_id": seriesID,
		"book_id":   input.BookID,
		"position":  input.Position,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
			return ErrBookAlreadyInSeries
		}
		return fmt.Errorf("add book to series: %w", err)
	}

	return nil
}

// RemoveBook removes a book from a series.
func (s *Service) RemoveBook(ctx context.Context, seriesID, bookID int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM series_books WHERE series_id = ? AND book_id = ?", seriesID, bookID)
	if err != nil {
		return fmt.Errorf("remove book from series: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted rows: %w", err)
	}
	if rows == 0 {
		return ErrBookNotFound
	}

	return nil
}

// UpdateBookPosition updates the position of a book in a series.
func (s *Service) UpdateBookPosition(ctx context.Context, seriesID, bookID int64, position string) error {
	result, err := s.db.NamedExecContext(ctx, `
		UPDATE series_books SET position = :position WHERE series_id = :series_id AND book_id = :book_id
	`, map[string]any{
		"position":  position,
		"series_id": seriesID,
		"book_id":   bookID,
	})
	if err != nil {
		return fmt.Errorf("update book position: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated rows: %w", err)
	}
	if rows == 0 {
		return ErrBookNotFound
	}

	return nil
}
