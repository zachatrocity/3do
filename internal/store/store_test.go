package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueItemLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	printer, err := db.CreatePrinter(ctx, PrinterInput{Name: "Voron", Location: "Bench", Active: true})
	if err != nil {
		t.Fatal(err)
	}

	item, err := db.CreateQueueItem(ctx, QueueItemInput{
		Title:       "Gridfinity bins",
		Status:      StatusQueued,
		Priority:    PriorityHigh,
		RequestedBy: "Zach",
		Owner:       "Shop",
		PrinterID:   &printer.ID,
		Quantity:    3,
		Material:    "PLA",
		Color:       "Black",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == 0 {
		t.Fatal("expected item id")
	}

	if _, err := db.AddLink(ctx, item.ID, "https://www.printables.com/model/example"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddFile(ctx, ItemFile{
		QueueItemID:  item.ID,
		StoragePath:  "uploads/1/example.stl",
		OriginalName: "example.stl",
		SizeBytes:    42,
		Checksum:     "abc123",
		ContentType:  "model/stl",
		Kind:         DetectFileKind("example.stl"),
	}); err != nil {
		t.Fatal(err)
	}

	items, err := db.ListQueueItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(items[0].Links))
	}
	if len(items[0].Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(items[0].Files))
	}
}

func TestQueueItemDetailUpdateNotesAndStatusHistory(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	item, err := db.CreateQueueItem(ctx, QueueItemInput{
		Title:    "Panel clips",
		Status:   StatusQueued,
		Priority: PriorityNormal,
		Owner:    "Shop",
		Quantity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	dueAt := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	estimate := 95
	updated, err := db.UpdateQueueItem(ctx, item.ID, QueueItemUpdate{
		Status:           StatusPrinting,
		Priority:         PriorityHigh,
		Owner:            "Alex",
		PrintingBy:       "Prusa MK4",
		Quantity:         4,
		Material:         "PETG",
		Color:            "Orange",
		EstimatedMinutes: &estimate,
		DueAt:            &dueAt,
		Actor:            "Admin",
		Note:             "Started print",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusPrinting || updated.Priority != PriorityHigh || updated.Owner != "Alex" {
		t.Fatalf("unexpected updated item: %+v", updated.QueueItem)
	}
	if updated.EstimatedMinutes == nil || *updated.EstimatedMinutes != estimate {
		t.Fatalf("expected estimate %d, got %+v", estimate, updated.EstimatedMinutes)
	}

	if _, err := db.AddNote(ctx, item.ID, "Admin", "First layer looks good."); err != nil {
		t.Fatal(err)
	}
	detail, err := db.GetQueueItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Notes) != 1 || detail.Notes[0].Body != "First layer looks good." {
		t.Fatalf("expected hydrated note, got %+v", detail.Notes)
	}
	if len(detail.StatusEvents) != 2 {
		t.Fatalf("expected create and update status events, got %d", len(detail.StatusEvents))
	}
	last := detail.StatusEvents[1]
	if last.OldStatus != StatusQueued || last.NewStatus != StatusPrinting || last.Actor != "Admin" {
		t.Fatalf("unexpected status event: %+v", last)
	}

	if _, err := db.UpdateQueueItem(ctx, item.ID, QueueItemUpdate{
		Status:   StatusPrinting,
		Priority: PriorityHigh,
		Owner:    "Alex",
		Quantity: 4,
	}); err != nil {
		t.Fatal(err)
	}
	detail, err = db.GetQueueItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.StatusEvents) != 2 {
		t.Fatalf("unchanged status should not add history, got %d events", len(detail.StatusEvents))
	}
}

func TestUploadAllowlist(t *testing.T) {
	for _, name := range []string{"part.stl", "plate.3mf", "job.gcode", "preview.png", "source.zip"} {
		if !AllowedUpload(name) {
			t.Fatalf("expected %s to be allowed", name)
		}
	}
	if AllowedUpload("notes.exe") {
		t.Fatal("exe upload should not be allowed")
	}
}

func TestNormalizeLinkAndSourceType(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantURL    string
		wantSource string
	}{
		{
			name:       "printables",
			input:      " HTTPS://WWW.PRINTABLES.COM/model/123-widget#files ",
			wantURL:    "https://www.printables.com/model/123-widget",
			wantSource: "printables",
		},
		{
			name:       "makerworld without scheme",
			input:      "makerworld.com/en/models/123",
			wantURL:    "https://makerworld.com/en/models/123",
			wantSource: "makerworld",
		},
		{
			name:       "thingiverse",
			input:      "https://www.thingiverse.com/thing:123",
			wantURL:    "https://www.thingiverse.com/thing:123",
			wantSource: "thingiverse",
		},
		{
			name:       "github",
			input:      "https://github.com/example/model",
			wantURL:    "https://github.com/example/model",
			wantSource: "github",
		},
		{
			name:       "direct url",
			input:      "https://example.com/file.stl",
			wantURL:    "https://example.com/file.stl",
			wantSource: "direct",
		},
		{
			name:       "other text",
			input:      "local NAS folder",
			wantURL:    "local NAS folder",
			wantSource: "other",
		},
		{
			name:       "similar domain is direct",
			input:      "https://notprintables.com/model/1",
			wantURL:    "https://notprintables.com/model/1",
			wantSource: "direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotSource, err := NormalizeLink(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if gotURL != tt.wantURL {
				t.Fatalf("expected URL %q, got %q", tt.wantURL, gotURL)
			}
			if gotSource != tt.wantSource {
				t.Fatalf("expected source %q, got %q", tt.wantSource, gotSource)
			}
		})
	}
}

func TestNormalizeLinkRejectsInvalidURLs(t *testing.T) {
	for _, input := range []string{"https://example.com/has space", "ftp://example.com/model.stl", "https:///missing-host"} {
		if _, _, err := NormalizeLink(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestLinkThumbnailStateRoundTrips(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateQueueItem(ctx, QueueItemInput{Title: "Bracket"})
	if err != nil {
		t.Fatal(err)
	}
	link, err := db.AddLink(ctx, item.ID, "https://www.thingiverse.com/thing:123")
	if err != nil {
		t.Fatal(err)
	}

	checkedAt := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	updated, err := db.UpdateLinkThumbnail(ctx, link.ID, LinkThumbnailUpdate{
		Title:                "Bracket preview",
		PreviewImageURL:      "https://cdn.example.test/preview.jpg",
		PreviewImageSource:   "og:image",
		ThumbnailPath:        "thumbnails/1-preview.jpg",
		ThumbnailContentType: "image/jpeg",
		ThumbnailStatus:      "ready",
		CheckedAt:            checkedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Bracket preview" || updated.ThumbnailStatus != "ready" {
		t.Fatalf("unexpected updated link: %+v", updated)
	}
	if updated.ThumbnailCheckedAt == nil || !updated.ThumbnailCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected checked_at to round trip, got %+v", updated.ThumbnailCheckedAt)
	}

	detail, err := db.GetQueueItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Links) != 1 || detail.Links[0].PreviewImageURL == "" || detail.Links[0].ThumbnailPath == "" {
		t.Fatalf("expected hydrated thumbnail metadata, got %+v", detail.Links)
	}
}

func TestFileByChecksum(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateQueueItem(ctx, QueueItemInput{Title: "Bracket"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddFile(ctx, ItemFile{
		QueueItemID:  item.ID,
		StoragePath:  "uploads/1/bracket.stl",
		OriginalName: "bracket.stl",
		SizeBytes:    128,
		Checksum:     "abc123",
		Kind:         "stl",
	}); err != nil {
		t.Fatal(err)
	}
	file, err := db.FileByChecksum(ctx, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if file.OriginalName != "bracket.stl" {
		t.Fatalf("expected bracket.stl, got %s", file.OriginalName)
	}
	if _, err := db.FileByChecksum(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
