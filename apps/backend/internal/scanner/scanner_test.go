package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"novil/backend/internal/testutil"
)

func TestScan(t *testing.T) {
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 10.cbz"), "01.jpg")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "Vol 2.cbz"), "01.jpg", "02.png", "__MACOSX/01.jpg", "notes.txt")
	testutil.WriteCBZ(t, filepath.Join(root, "Berserk", "extras", "Side Story.cbz"), "a.jpg")
	testutil.WriteFile(t, filepath.Join(root, "Dune.epub"), "not really an epub")
	testutil.WriteFile(t, filepath.Join(root, "Berserk", "cover.jpg"), "ignored")
	testutil.WriteFile(t, filepath.Join(root, ".hidden", "Secret.cbz"), "ignored")
	testutil.WriteFile(t, filepath.Join(root, "Broken", "bad.cbz"), "not a zip")

	series, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 {
		t.Fatalf("got %d series, want 2: %+v", len(series), series)
	}

	berserk := series[0]
	if berserk.Name != "Berserk" || len(berserk.Books) != 3 {
		t.Fatalf("unexpected first series: %+v", berserk)
	}
	// Books sort by path, so the nested folder comes first.
	wantTitles := []string{"Side Story", "Vol 2", "Vol 10"}
	for i, b := range berserk.Books {
		if b.Title != wantTitles[i] {
			t.Errorf("book %d: title %q, want %q", i, b.Title, wantTitles[i])
		}
	}
	if got := berserk.Books[1].PageCount; got != 2 {
		t.Errorf("Vol 2 page count = %d, want 2", got)
	}

	dune := series[1]
	if dune.Name != "Dune" || len(dune.Books) != 1 || dune.Books[0].Format != "epub" {
		t.Errorf("unexpected second series: %+v", dune)
	}
}

func TestScanSkipsUnreadableFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read every folder")
	}
	root := t.TempDir()
	testutil.WriteCBZ(t, filepath.Join(root, "Readable", "1.cbz"), "01.jpg")
	locked := filepath.Join(root, "Locked")
	testutil.WriteCBZ(t, filepath.Join(locked, "1.cbz"), "01.jpg")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	series, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].Name != "Readable" {
		t.Errorf("got %+v, want only Readable", series)
	}
}
