package book

import (
	"context"
	"fmt"
	"strings"
)

// GetWithEditions returns a book with its editions and files.
func (s *Service) GetWithEditions(ctx context.Context, id int64) (*BookWithEditions, error) {
	book, err := s.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := &BookWithEditions{
		Book:     *book,
		Editions: []Edition{},
		Files:    []BookFile{},
	}

	if err := s.db.SelectContext(ctx, &result.Editions, `
		SELECT id, book_id, foreign_id, title, isbn, isbn13, format, publisher, release_date, page_count, language, monitored
		FROM editions WHERE book_id = ?
	`, id); err != nil {
		return nil, fmt.Errorf("get editions: %w", err)
	}

	if err := s.db.SelectContext(ctx, &result.Files, `
		SELECT id, book_id, edition_id, path, relative_path, size, format, quality, hash, added_at, content_mismatch
		FROM book_files WHERE book_id = ?
	`, id); err != nil {
		return nil, fmt.Errorf("get book files: %w", err)
	}

	return result, nil
}

// CreateEdition creates a new edition for a book.
func (s *Service) CreateEdition(ctx context.Context, input CreateEditionInput) (*Edition, error) {
	if input.BookID == 0 || input.Title == "" {
		return nil, ErrInvalidInput
	}

	// Check book exists
	_, err := s.FindByID(ctx, input.BookID)
	if err != nil {
		return nil, err
	}

	monitored := 0
	if input.Monitored {
		monitored = 1
	}

	result, err := s.db.NamedExecContext(ctx, `
		INSERT INTO editions (book_id, foreign_id, title, isbn, isbn13, format, publisher, release_date, page_count, language, monitored)
		VALUES (:book_id, :foreign_id, :title, :isbn, :isbn13, :format, :publisher, :release_date, :page_count, :language, :monitored)
	`, map[string]any{
		"book_id":      input.BookID,
		"foreign_id":   input.ForeignID,
		"title":        input.Title,
		"isbn":         input.ISBN,
		"isbn13":       input.ISBN13,
		"format":       input.Format,
		"publisher":    input.Publisher,
		"release_date": input.ReleaseDate,
		"page_count":   input.PageCount,
		"language":     input.Language,
		"monitored":    monitored,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create edition: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get edition id: %w", err)
	}

	var e Edition
	err = s.db.GetContext(ctx, &e, `
		SELECT id, book_id, foreign_id, title, isbn, isbn13, format, publisher, release_date, page_count, language, monitored
		FROM editions WHERE id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("get created edition: %w", err)
	}

	return &e, nil
}

// DeleteEdition deletes an edition by ID.
func (s *Service) DeleteEdition(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM editions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete edition %d: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted rows: %w", err)
	}
	if rows == 0 {
		return ErrEditionNotFound
	}

	return nil
}
