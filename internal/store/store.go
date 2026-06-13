package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			display_name TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'member',
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS printers (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			location TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'idle',
			capabilities TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS queue_items (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'backlog',
			priority TEXT NOT NULL DEFAULT 'normal',
			requested_by TEXT NOT NULL DEFAULT '',
			owner TEXT NOT NULL DEFAULT '',
			printing_by TEXT NOT NULL DEFAULT '',
			printer_id INTEGER REFERENCES printers(id) ON DELETE SET NULL,
			quantity INTEGER NOT NULL DEFAULT 1,
			material TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '',
			estimated_minutes INTEGER,
			due_at TEXT,
			reprint_of_id INTEGER REFERENCES queue_items(id) ON DELETE SET NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (status IN ('backlog', 'queued', 'printing', 'blocked', 'done', 'cancelled')),
			CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
			CHECK (quantity > 0)
		)`,
		`CREATE TABLE IF NOT EXISTS item_links (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			url TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'other',
			title TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS item_files (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			storage_path TEXT NOT NULL,
			original_name TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			checksum TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'other',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS item_notes (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			author TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS status_events (
			id INTEGER PRIMARY KEY,
			queue_item_id INTEGER NOT NULL REFERENCES queue_items(id) ON DELETE CASCADE,
			old_status TEXT NOT NULL DEFAULT '',
			new_status TEXT NOT NULL,
			actor TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS queue_items_status_idx ON queue_items(status)`,
		`CREATE INDEX IF NOT EXISTS queue_items_printer_idx ON queue_items(printer_id)`,
		`CREATE INDEX IF NOT EXISTS item_files_checksum_idx ON item_files(checksum)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateQueueItem(ctx context.Context, input QueueItemInput) (QueueItem, error) {
	normalizeQueueInput(&input)
	if strings.TrimSpace(input.Title) == "" {
		return QueueItem{}, errors.New("title is required")
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO queue_items (
		title, description, status, priority, requested_by, owner, printing_by, printer_id,
		quantity, material, color, estimated_minutes, due_at, reprint_of_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id, title, description, status, priority, requested_by, owner, printing_by, printer_id,
		quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at`,
		input.Title, input.Description, input.Status, input.Priority, input.RequestedBy, input.Owner, input.PrintingBy,
		input.PrinterID, input.Quantity, input.Material, input.Color, input.EstimatedMinutes, timePtrString(input.DueAt), input.ReprintOfID)
	item, err := scanQueueItem(row)
	if err != nil {
		return QueueItem{}, err
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO status_events (queue_item_id, new_status, actor, note) VALUES (?, ?, ?, ?)`,
		item.ID, item.Status, item.Owner, "created")
	return item, nil
}

func (s *Store) ListQueueItems(ctx context.Context) ([]QueueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, description, status, priority, requested_by, owner, printing_by,
		printer_id, quantity, material, color, estimated_minutes, due_at, reprint_of_id, created_at, updated_at
		FROM queue_items ORDER BY
		CASE status
			WHEN 'printing' THEN 1
			WHEN 'queued' THEN 2
			WHEN 'blocked' THEN 3
			WHEN 'backlog' THEN 4
			WHEN 'done' THEN 5
			ELSE 6
		END, priority DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		item, err := scanQueueItem(rows)
		if err != nil {
			return nil, err
		}
		item.Links, err = s.ListLinks(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Files, err = s.ListFiles(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AddLink(ctx context.Context, queueItemID int64, rawURL string) (ItemLink, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ItemLink{}, errors.New("url is required")
	}
	sourceType := detectSourceType(rawURL)
	row := s.db.QueryRowContext(ctx, `INSERT INTO item_links (queue_item_id, url, source_type)
		VALUES (?, ?, ?)
		RETURNING id, queue_item_id, url, source_type, title, created_at`, queueItemID, rawURL, sourceType)
	return scanLink(row)
}

func (s *Store) ListLinks(ctx context.Context, queueItemID int64) ([]ItemLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, url, source_type, title, created_at
		FROM item_links WHERE queue_item_id = ? ORDER BY created_at ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []ItemLink
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) AddFile(ctx context.Context, file ItemFile) (ItemFile, error) {
	row := s.db.QueryRowContext(ctx, `INSERT INTO item_files
		(queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind, created_at`,
		file.QueueItemID, file.StoragePath, file.OriginalName, file.SizeBytes, file.Checksum, file.ContentType, file.Kind)
	return scanFile(row)
}

func (s *Store) ListFiles(ctx context.Context, queueItemID int64) ([]ItemFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, queue_item_id, storage_path, original_name, size_bytes, checksum, content_type, kind, created_at
		FROM item_files WHERE queue_item_id = ? ORDER BY created_at ASC`, queueItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []ItemFile
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (s *Store) CreatePrinter(ctx context.Context, input PrinterInput) (Printer, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Printer{}, errors.New("name is required")
	}
	if input.Status == "" {
		input.Status = "idle"
	}
	row := s.db.QueryRowContext(ctx, `INSERT INTO printers (name, location, status, capabilities, notes, active)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, name, location, status, capabilities, notes, active, created_at, updated_at`,
		input.Name, input.Location, input.Status, input.Capabilities, input.Notes, boolInt(input.Active))
	return scanPrinter(row)
}

func (s *Store) ListPrinters(ctx context.Context) ([]Printer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, location, status, capabilities, notes, active, created_at, updated_at
		FROM printers ORDER BY active DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var printers []Printer
	for rows.Next() {
		printer, err := scanPrinter(rows)
		if err != nil {
			return nil, err
		}
		printers = append(printers, printer)
	}
	return printers, rows.Err()
}

func normalizeQueueInput(input *QueueItemInput) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Status == "" {
		input.Status = StatusBacklog
	}
	if input.Priority == "" {
		input.Priority = PriorityNormal
	}
	if input.Quantity < 1 {
		input.Quantity = 1
	}
}

func detectSourceType(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "other"
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.Contains(host, "printables.com"):
		return "printables"
	case strings.Contains(host, "makerworld.com"):
		return "makerworld"
	case strings.Contains(host, "thingiverse.com"):
		return "thingiverse"
	case strings.Contains(host, "github.com"):
		return "github"
	case parsed.Scheme == "http" || parsed.Scheme == "https":
		return "direct"
	default:
		return "other"
	}
}

func DetectFileKind(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".stl":
		return "stl"
	case ".3mf":
		return "3mf"
	case ".gcode":
		return "gcode"
	case ".step", ".stp":
		return "step"
	case ".obj":
		return "obj"
	case ".png", ".jpg", ".jpeg", ".webp":
		return "image"
	default:
		return "other"
	}
}

func AllowedUpload(name string) bool {
	switch DetectFileKind(name) {
	case "stl", "3mf", "gcode", "step", "obj", "image":
		return true
	default:
		return strings.EqualFold(filepath.Ext(name), ".zip")
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQueueItem(row scanner) (QueueItem, error) {
	var item QueueItem
	var printerID, estimatedMinutes, reprintOfID sql.NullInt64
	var dueAt sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&item.ID, &item.Title, &item.Description, &item.Status, &item.Priority,
		&item.RequestedBy, &item.Owner, &item.PrintingBy, &printerID, &item.Quantity, &item.Material,
		&item.Color, &estimatedMinutes, &dueAt, &reprintOfID, &createdAt, &updatedAt)
	if err != nil {
		return QueueItem{}, err
	}
	if printerID.Valid {
		value := printerID.Int64
		item.PrinterID = &value
	}
	if estimatedMinutes.Valid {
		value := int(estimatedMinutes.Int64)
		item.EstimatedMinutes = &value
	}
	if reprintOfID.Valid {
		value := reprintOfID.Int64
		item.ReprintOfID = &value
	}
	if dueAt.Valid {
		parsed, err := time.Parse(time.RFC3339, dueAt.String)
		if err == nil {
			item.DueAt = &parsed
		}
	}
	item.CreatedAt = parseSQLiteTime(createdAt)
	item.UpdatedAt = parseSQLiteTime(updatedAt)
	return item, nil
}

func scanLink(row scanner) (ItemLink, error) {
	var link ItemLink
	var createdAt string
	if err := row.Scan(&link.ID, &link.QueueItemID, &link.URL, &link.SourceType, &link.Title, &createdAt); err != nil {
		return ItemLink{}, err
	}
	link.CreatedAt = parseSQLiteTime(createdAt)
	return link, nil
}

func scanFile(row scanner) (ItemFile, error) {
	var file ItemFile
	var createdAt string
	if err := row.Scan(&file.ID, &file.QueueItemID, &file.StoragePath, &file.OriginalName, &file.SizeBytes, &file.Checksum, &file.ContentType, &file.Kind, &createdAt); err != nil {
		return ItemFile{}, err
	}
	file.CreatedAt = parseSQLiteTime(createdAt)
	return file, nil
}

func scanPrinter(row scanner) (Printer, error) {
	var printer Printer
	var active int
	var createdAt, updatedAt string
	if err := row.Scan(&printer.ID, &printer.Name, &printer.Location, &printer.Status, &printer.Capabilities, &printer.Notes, &active, &createdAt, &updatedAt); err != nil {
		return Printer{}, err
	}
	printer.Active = active == 1
	printer.CreatedAt = parseSQLiteTime(createdAt)
	printer.UpdatedAt = parseSQLiteTime(updatedAt)
	return printer, nil
}

func timePtrString(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) DebugString() string {
	return fmt.Sprintf("store{%p}", s.db)
}
