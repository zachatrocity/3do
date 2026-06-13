package store

import (
	"context"
	"path/filepath"
	"testing"
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
