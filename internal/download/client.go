package download

import (
	"context"
	"time"
)

// Client defines the interface for download clients.
type Client interface {
	Name() string
	Type() string
	Test(ctx context.Context) error
	Add(ctx context.Context, item AddItem) (string, error)
	Remove(ctx context.Context, id string, deleteData bool) error
	GetStatus(ctx context.Context, id string) (*ItemStatus, error)
	GetQueue(ctx context.Context) ([]ItemStatus, error)
}

// AddItem represents an item to be added to a download client.
type AddItem struct {
	Name        string            `json:"name"`
	DownloadURL string            `json:"downloadUrl"`
	Category    string            `json:"category,omitempty"`
	Priority    Priority          `json:"priority,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	SavePath    string            `json:"savePath,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// ItemStatus represents the status of a download item.
type ItemStatus struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Status         Status        `json:"status"`
	Progress       float64       `json:"progress"`
	Size           int64         `json:"size"`
	DownloadedSize int64         `json:"downloadedSize"`
	Speed          int64         `json:"speed"`
	ETA            time.Duration `json:"eta"`
	Seeders        int           `json:"seeders,omitempty"`
	Leechers       int           `json:"leechers,omitempty"`
	Ratio          float64       `json:"ratio,omitempty"`
	SavePath       string        `json:"savePath,omitempty"`
	Category       string        `json:"category,omitempty"`
	ErrorMessage   string        `json:"errorMessage,omitempty"`
	AddedAt        time.Time     `json:"addedAt"`
	CompletedAt    *time.Time    `json:"completedAt,omitempty"`
}

// Status represents the current status of a download.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusPaused      Status = "paused"
	StatusCompleted   Status = "completed"
	StatusSeeding     Status = "seeding"
	StatusFailed      Status = "failed"
	StatusExtracted   Status = "extracted"
	StatusProcessing  Status = "processing"
)

// Priority represents download priority.
type Priority int

const (
	PriorityLow    Priority = -1
	PriorityNormal Priority = 0
	PriorityHigh   Priority = 1
	PriorityForced Priority = 2
)

// ClientConfig holds the configuration for a download client.
type ClientConfig struct {
	ID                   int64    `json:"id"                    db:"id"`
	Name                 string   `json:"name"                  db:"name"`
	Type                 string   `json:"type"                  db:"type"`
	Host                 string   `json:"host"                  db:"host"`
	Port                 int      `json:"port"                  db:"port"`
	UseTLS               bool     `json:"useTls"                db:"use_tls"`
	Username             string   `json:"username,omitempty"    db:"username"`
	Password             string   `json:"password,omitempty"    db:"password"`
	APIKey               string   `json:"apiKey,omitempty"      db:"api_key"`
	Category             string   `json:"category,omitempty"    db:"category"`
	RecentPriority       Priority `json:"recentPriority"        db:"recent_priority"`
	OlderPriority        Priority `json:"olderPriority"         db:"older_priority"`
	RemoveCompletedAfter int      `json:"removeCompletedAfter"  db:"remove_completed_after"`
	Enabled              bool     `json:"enabled"               db:"enabled"`
	Priority             int      `json:"priority"              db:"priority"`
	NzbFolder            string   `json:"nzbFolder,omitempty"   db:"nzb_folder"`
	TorrentFolder        string   `json:"torrentFolder,omitempty" db:"torrent_folder"`
	WatchFolder          string   `json:"watchFolder,omitempty" db:"watch_folder"`
	DownloadDir          string   `json:"downloadDir,omitempty" db:"download_dir"` // For direct downloads
	CreatedAt            string   `json:"createdAt"             db:"created_at"`
	UpdatedAt            string   `json:"updatedAt"             db:"updated_at"`
}

// ClientType values.
const (
	ClientTypeSABnzbd      = "sabnzbd"
	ClientTypeQBittorrent  = "qbittorrent"
	ClientTypeTransmission = "transmission"
	ClientTypeBlackhole    = "blackhole"
	ClientTypeDirect       = "direct"
)

// GrabStatus represents the status of a grab.
type GrabStatus string

const (
	GrabStatusPending     GrabStatus = "pending"
	GrabStatusSent        GrabStatus = "sent"
	GrabStatusDownloading GrabStatus = "downloading"
	GrabStatusCompleted   GrabStatus = "completed"
	GrabStatusFailed      GrabStatus = "failed"
	GrabStatusImported    GrabStatus = "imported"
)

// GrabItem represents a grabbed item waiting for download.
type GrabItem struct {
	ID           int64      `json:"id"`
	BookID       int64      `json:"bookId"`
	IndexerID    int64      `json:"indexerId"`
	ReleaseTitle string     `json:"releaseTitle"`
	DownloadURL  string     `json:"downloadUrl"`
	Size         int64      `json:"size"`
	Quality      string     `json:"quality"`
	ClientID     int64      `json:"clientId"`
	DownloadID   string     `json:"downloadId"`
	Status       GrabStatus `json:"status"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	GrabbedAt    time.Time  `json:"grabbedAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}
