package store

import "time"

type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleMember UserRole = "member"
)

type User struct {
	ID           int64     `json:"id"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	Role         UserRole  `json:"role"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	PasswordHash string    `json:"-"`
}

type UserInput struct {
	DisplayName  string
	Email        string
	Role         UserRole
	Active       bool
	PasswordHash string
}

type UserUpdate struct {
	DisplayName  string
	Email        string
	Role         UserRole
	Active       bool
	PasswordHash string
}

type QueueStatus string

const (
	StatusBacklog   QueueStatus = "backlog"
	StatusQueued    QueueStatus = "queued"
	StatusPrinting  QueueStatus = "printing"
	StatusBlocked   QueueStatus = "blocked"
	StatusDone      QueueStatus = "done"
	StatusCancelled QueueStatus = "cancelled"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type QueueItem struct {
	ID               int64       `json:"id"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	Status           QueueStatus `json:"status"`
	Priority         Priority    `json:"priority"`
	RequestedBy      string      `json:"requested_by"`
	Owner            string      `json:"owner"`
	PrintingBy       string      `json:"printing_by"`
	PrinterID        *int64      `json:"printer_id,omitempty"`
	Quantity         int         `json:"quantity"`
	Material         string      `json:"material"`
	Color            string      `json:"color"`
	EstimatedMinutes *int        `json:"estimated_minutes,omitempty"`
	DueAt            *time.Time  `json:"due_at,omitempty"`
	ReprintOfID      *int64      `json:"reprint_of_id,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Links            []ItemLink  `json:"links,omitempty"`
	Files            []ItemFile  `json:"files,omitempty"`
}

type QueueItemDetail struct {
	QueueItem
	Notes        []ItemNote    `json:"notes"`
	StatusEvents []StatusEvent `json:"status_events"`
}

type QueueItemInput struct {
	Title            string
	Description      string
	Status           QueueStatus
	Priority         Priority
	RequestedBy      string
	Owner            string
	PrintingBy       string
	PrinterID        *int64
	Quantity         int
	Material         string
	Color            string
	EstimatedMinutes *int
	DueAt            *time.Time
	ReprintOfID      *int64
}

type QueueItemUpdate struct {
	Status           QueueStatus
	Priority         Priority
	Owner            string
	PrintingBy       string
	Quantity         int
	Material         string
	Color            string
	EstimatedMinutes *int
	DueAt            *time.Time
	Actor            string
	Note             string
}

type ItemNote struct {
	ID          int64     `json:"id"`
	QueueItemID int64     `json:"queue_item_id"`
	Author      string    `json:"author"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type StatusEvent struct {
	ID          int64       `json:"id"`
	QueueItemID int64       `json:"queue_item_id"`
	OldStatus   QueueStatus `json:"old_status"`
	NewStatus   QueueStatus `json:"new_status"`
	Actor       string      `json:"actor"`
	Note        string      `json:"note"`
	CreatedAt   time.Time   `json:"created_at"`
}

type ItemLink struct {
	ID                   int64      `json:"id"`
	QueueItemID          int64      `json:"queue_item_id"`
	URL                  string     `json:"url"`
	SourceType           string     `json:"source_type"`
	Title                string     `json:"title"`
	PreviewImageURL      string     `json:"preview_image_url,omitempty"`
	PreviewImageSource   string     `json:"preview_image_source,omitempty"`
	ThumbnailPath        string     `json:"-"`
	ThumbnailContentType string     `json:"thumbnail_content_type,omitempty"`
	ThumbnailStatus      string     `json:"thumbnail_status,omitempty"`
	ThumbnailCheckedAt   *time.Time `json:"thumbnail_checked_at,omitempty"`
	ThumbnailError       string     `json:"thumbnail_error,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type LinkThumbnailUpdate struct {
	Title                string
	PreviewImageURL      string
	PreviewImageSource   string
	ThumbnailPath        string
	ThumbnailContentType string
	ThumbnailStatus      string
	ThumbnailError       string
	CheckedAt            time.Time
}

type ItemFile struct {
	ID           int64     `json:"id"`
	QueueItemID  int64     `json:"queue_item_id"`
	StoragePath  string    `json:"storage_path"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	Checksum     string    `json:"checksum"`
	ContentType  string    `json:"content_type"`
	Kind         string    `json:"kind"`
	CreatedAt    time.Time `json:"created_at"`
}

type Printer struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Location     string    `json:"location"`
	Status       string    `json:"status"`
	Capabilities string    `json:"capabilities"`
	Notes        string    `json:"notes"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PrinterInput struct {
	Name         string
	Location     string
	Status       string
	Capabilities string
	Notes        string
	Active       bool
}
