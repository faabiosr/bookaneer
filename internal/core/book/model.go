package book

// Book represents a book in the library.
type Book struct {
	ID          int64  `json:"id"          db:"id"`
	AuthorID    int64  `json:"authorId"    db:"author_id"`
	Title       string `json:"title"       db:"title"`
	SortTitle   string `json:"sortTitle"   db:"sort_title"`
	ForeignID   string `json:"foreignId"   db:"foreign_id"` // OpenLibrary work key
	ISBN        string `json:"isbn"        db:"isbn"`
	ISBN13      string `json:"isbn13"      db:"isbn13"`
	ReleaseDate string `json:"releaseDate" db:"release_date"` // YYYY-MM-DD
	Overview    string `json:"overview"    db:"overview"`
	ImageURL    string `json:"imageUrl"    db:"image_url"`
	PageCount   int    `json:"pageCount"   db:"page_count"`
	Monitored   bool   `json:"monitored"   db:"monitored"`
	AddedAt     string `json:"addedAt"     db:"added_at"`
	UpdatedAt   string `json:"updatedAt"   db:"updated_at"`

	// Computed/joined fields
	AuthorName string `json:"authorName,omitempty" db:"name"`
	HasFile    bool   `json:"hasFile,omitempty"    db:"has_file"`
	FileFormat string `json:"fileFormat,omitempty" db:"file_format"` // format of the primary file on disk
}

// Edition represents a specific edition of a book.
type Edition struct {
	ID          int64  `json:"id"          db:"id"`
	BookID      int64  `json:"bookId"      db:"book_id"`
	ForeignID   string `json:"foreignId"   db:"foreign_id"` // OpenLibrary edition key
	Title       string `json:"title"       db:"title"`
	ISBN        string `json:"isbn"        db:"isbn"`
	ISBN13      string `json:"isbn13"      db:"isbn13"`
	Format      string `json:"format"      db:"format"` // epub, mobi, pdf, hardcover, paperback
	Publisher   string `json:"publisher"   db:"publisher"`
	ReleaseDate string `json:"releaseDate" db:"release_date"`
	PageCount   int    `json:"pageCount"   db:"page_count"`
	Language    string `json:"language"    db:"language"` // ISO 639-1
	Monitored   bool   `json:"monitored"   db:"monitored"`
}

// BookFile represents a book file on disk.
type BookFile struct {
	ID              int64  `json:"id"              db:"id"`
	BookID          int64  `json:"bookId"          db:"book_id"`
	EditionID       *int64 `json:"editionId,omitempty" db:"edition_id"`
	Path            string `json:"path"            db:"path"`
	RelativePath    string `json:"relativePath"    db:"relative_path"`
	Size            int64  `json:"size"            db:"size"`
	Format          string `json:"format"          db:"format"` // epub, mobi, azw3, pdf, cbz
	Quality         string `json:"quality"         db:"quality"`
	Hash            string `json:"hash"            db:"hash"` // SHA-256
	AddedAt         string `json:"addedAt"         db:"added_at"`
	ContentMismatch bool   `json:"contentMismatch" db:"content_mismatch"`
}

// CreateBookInput holds the data needed to create a new book.
type CreateBookInput struct {
	AuthorID    int64  `json:"authorId"`
	Title       string `json:"title"`
	SortTitle   string `json:"sortTitle"`
	ForeignID   string `json:"foreignId"`
	ISBN        string `json:"isbn"`
	ISBN13      string `json:"isbn13"`
	ReleaseDate string `json:"releaseDate"`
	Overview    string `json:"overview"`
	ImageURL    string `json:"imageUrl"`
	PageCount   int    `json:"pageCount"`
	Monitored   bool   `json:"monitored"`
}

// UpdateBookInput holds the data for updating an existing book.
type UpdateBookInput struct {
	AuthorID    *int64  `json:"authorId,omitempty"`
	Title       *string `json:"title,omitempty"`
	SortTitle   *string `json:"sortTitle,omitempty"`
	ForeignID   *string `json:"foreignId,omitempty"`
	ISBN        *string `json:"isbn,omitempty"`
	ISBN13      *string `json:"isbn13,omitempty"`
	ReleaseDate *string `json:"releaseDate,omitempty"`
	Overview    *string `json:"overview,omitempty"`
	ImageURL    *string `json:"imageUrl,omitempty"`
	PageCount   *int    `json:"pageCount,omitempty"`
	Monitored   *bool   `json:"monitored,omitempty"`
}

// CreateEditionInput holds the data needed to create a new edition.
type CreateEditionInput struct {
	BookID      int64  `json:"bookId"`
	ForeignID   string `json:"foreignId"`
	Title       string `json:"title"`
	ISBN        string `json:"isbn"`
	ISBN13      string `json:"isbn13"`
	Format      string `json:"format"`
	Publisher   string `json:"publisher"`
	ReleaseDate string `json:"releaseDate"`
	PageCount   int    `json:"pageCount"`
	Language    string `json:"language"`
	Monitored   bool   `json:"monitored"`
}

// ListBooksFilter provides filtering options for listing books.
type ListBooksFilter struct {
	AuthorID  *int64
	Monitored *bool
	Missing   bool // Only books without files
	Search    string
	SortBy    string // title, sortTitle, releaseDate, addedAt
	SortDir   string // asc, desc
	Limit     int
	Offset    int
}

// BookWithEditions represents a book with its editions.
type BookWithEditions struct {
	Book
	Editions []Edition  `json:"editions"`
	Files    []BookFile `json:"files,omitempty"`
}
