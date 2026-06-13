package store

import "time"

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

type ItemLink struct {
	ID          int64     `json:"id"`
	QueueItemID int64     `json:"queue_item_id"`
	URL         string    `json:"url"`
	SourceType  string    `json:"source_type"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
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
