package wanted

// GrabResult represents the result of a grab attempt.
type GrabResult struct {
	BookID       int64  `json:"bookId"`
	Title        string `json:"title"`
	Source       string `json:"source"` // "library" or "indexer"
	ProviderName string `json:"providerName"`
	Format       string `json:"format"`
	Size         int64  `json:"size"`
	DownloadID   string `json:"downloadId"` // ID from download client
	ClientName   string `json:"clientName"`
}

// DownloadQueueItem represents an item in the download queue.
type DownloadQueueItem struct {
	ID               int64   `json:"id"                        db:"id"`
	BookID           int64   `json:"bookId"                    db:"book_id"`
	DownloadClientID *int64  `json:"downloadClientId,omitempty" db:"download_client_id"`
	IndexerID        *int64  `json:"indexerId,omitempty"        db:"indexer_id"`
	ExternalID       string  `json:"externalId"                db:"external_id"`
	Title            string  `json:"title"                     db:"title"`
	Size             int64   `json:"size"                      db:"size"`
	Format           string  `json:"format"                    db:"format"`
	Status           string  `json:"status"                    db:"status"`
	Progress         float64 `json:"progress"                  db:"progress"`
	DownloadURL      string  `json:"downloadUrl"               db:"download_url"`
	AddedAt          string  `json:"addedAt"                   db:"added_at"`
	BookTitle        string  `json:"bookTitle"                 db:"book_title"`
	ClientName       string  `json:"clientName"                db:"client_name"`
}

// HistoryItem represents a history event.
type HistoryItem struct {
	ID          int64          `json:"id"                    db:"id"`
	BookID      *int64         `json:"bookId,omitempty"      db:"book_id"`
	AuthorID    *int64         `json:"authorId,omitempty"    db:"author_id"`
	EventType   string         `json:"eventType"             db:"event_type"`
	SourceTitle string         `json:"sourceTitle"           db:"source_title"`
	Quality     string         `json:"quality"               db:"quality"`
	Data        map[string]any `json:"data"                  db:"-"`
	Date        string         `json:"date"                  db:"date"`
	BookTitle   string         `json:"bookTitle,omitempty"   db:"book_title"`
	AuthorName  string         `json:"authorName,omitempty"  db:"author_name"`
}

// BlocklistItem represents a blocked release.
type BlocklistItem struct {
	ID          int64  `json:"id"          db:"id"`
	BookID      int64  `json:"bookId"      db:"book_id"`
	SourceTitle string `json:"sourceTitle" db:"source_title"`
	Quality     string `json:"quality"     db:"quality"`
	Reason      string `json:"reason"      db:"reason"`
	Date        string `json:"date"        db:"date"`
	BookTitle   string `json:"bookTitle"   db:"book_title"`
	AuthorName  string `json:"authorName"  db:"author_name"`
}
