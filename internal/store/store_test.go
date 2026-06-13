package store

import (
	"context"
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
