package metadata

import (
	"path/filepath"
	"testing"

	"novil/backend/internal/testutil"
)

func TestEPUB3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	testutil.WriteEPUB(t, path, `
		<dc:title>  Der   Hobbit </dc:title>
		<dc:creator id="a1">J. R. R. Tolkien</dc:creator>
		<meta refines="#a1" property="role" scheme="marc:relators">aut</meta>
		<dc:creator id="i1">Alan Lee</dc:creator>
		<meta refines="#i1" property="role" scheme="marc:relators">ill</meta>
		<dc:language>de</dc:language>
		<dc:date>1937-09-21T00:00:00Z</dc:date>
		<dc:description>&lt;p&gt;Ein &lt;b&gt;Abenteuer&lt;/b&gt;.&lt;/p&gt;&lt;p&gt;Zweiter &amp;amp; letzter Absatz&lt;/p&gt;</dc:description>
		<meta property="dcterms:modified">2020-01-01T00:00:00Z</meta>`)
	md, err := Read(path, "epub")
	if err != nil {
		t.Fatal(err)
	}
	want := Metadata{
		Title:       "Der Hobbit",
		Author:      "J. R. R. Tolkien",
		Language:    "de",
		Released:    "1937-09-21",
		Description: "Ein Abenteuer.\n\nZweiter & letzter Absatz",
	}
	if md != want {
		t.Errorf("got  %+v\nwant %+v", md, want)
	}
}

func TestEPUB2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	testutil.WriteEPUB(t, path, `
		<dc:title>Dune</dc:title>
		<dc:creator opf:role="edt">Some Editor</dc:creator>
		<dc:creator opf:role="aut">Frank Herbert</dc:creator>
		<dc:date opf:event="modification">2010-02-03</dc:date>
		<dc:date opf:event="publication">1965</dc:date>`)
	md, err := Read(path, "epub")
	if err != nil {
		t.Fatal(err)
	}
	if md.Author != "Frank Herbert" || md.Released != "1965" {
		t.Errorf("got %+v, want author Frank Herbert released 1965", md)
	}
}

func TestEPUBWithoutRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	testutil.WriteEPUB(t, path, `<dc:creator>Terry Pratchett</dc:creator><dc:creator>Neil Gaiman</dc:creator>`)
	md, err := Read(path, "epub")
	if err != nil {
		t.Fatal(err)
	}
	if md.Author != "Terry Pratchett, Neil Gaiman" || md.Title != "" {
		t.Errorf("got %+v", md)
	}
}

func TestBrokenEPUB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	testutil.WriteZip(t, path, map[string]string{"mimetype": "application/epub+zip"})
	if _, err := Read(path, "epub"); err == nil {
		t.Error("EPUB without container.xml: got nil error")
	}
}

func TestComicInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.cbz")
	testutil.WriteZip(t, path, map[string]string{
		"01.jpg": "",
		"ComicInfo.xml": `<?xml version="1.0"?>
<ComicInfo>
  <Title>The Black Swordsman</Title>
  <Writer>Kentaro Miura</Writer>
  <Year>1990</Year><Month>11</Month>
  <LanguageISO>en</LanguageISO>
  <Summary>Guts arrives.</Summary>
</ComicInfo>`,
	})
	md, err := Read(path, "cbz")
	if err != nil {
		t.Fatal(err)
	}
	want := Metadata{Title: "The Black Swordsman", Author: "Kentaro Miura", Language: "en", Released: "1990-11", Description: "Guts arrives."}
	if md != want {
		t.Errorf("got  %+v\nwant %+v", md, want)
	}

	plain := filepath.Join(t.TempDir(), "plain.cbz")
	testutil.WriteCBZ(t, plain, "01.jpg")
	if md, err := Read(plain, "cbz"); err != nil || md != (Metadata{}) {
		t.Errorf("cbz without ComicInfo: %+v, %v", md, err)
	}
}

func TestDates(t *testing.T) {
	for in, want := range map[string]string{
		"2019":                 "2019",
		"2019-05":              "2019-05",
		"2019-05-03":           "2019-05-03",
		"2019-05-03T10:00:00Z": "2019-05-03",
		" 1965 ":               "1965",
		"May 1965":             "",
		"":                     "",
	} {
		if got := NormalizeDate(in); got != want {
			t.Errorf("NormalizeDate(%q) = %q, want %q", in, got, want)
		}
	}
	if ValidDate("2019-05-03T10:00:00Z") || !ValidDate("2019-05") || !ValidDate("") {
		t.Error("ValidDate is wrong")
	}
}

// Calibre and others write HTML entities, which XML itself doesn't know.
func TestEPUBWithHTMLEntities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	testutil.WriteEPUB(t, path, `<dc:title>Feliz ano velho</dc:title><dc:description>It&rsquo;s a &eacute;pico.</dc:description>`)
	md, err := Read(path, "epub")
	if err != nil {
		t.Fatal(err)
	}
	if md.Title != "Feliz ano velho" || md.Description != "It’s a épico." {
		t.Errorf("got %+v", md)
	}
}

func TestISBN(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "book.epub")
	testutil.WriteEPUB(t, epub, `<dc:identifier>urn:uuid:0b8f8a1e-1111-2222-3333-444455556666</dc:identifier>
		<dc:identifier opf:scheme="ISBN">urn:isbn:0-441-01359-7</dc:identifier>`)
	if md, err := Read(epub, "epub"); err != nil || md.ISBN != "9780441013593" {
		t.Errorf("epub: %+v, %v", md, err)
	}
	// A number that isn't a valid ISBN is no ISBN.
	testutil.WriteEPUB(t, epub, `<dc:identifier>9780441013594</dc:identifier>`)
	if md, _ := Read(epub, "epub"); md.ISBN != "" {
		t.Errorf("invalid ISBN read: %q", md.ISBN)
	}
	cbz := filepath.Join(dir, "book.cbz")
	testutil.WriteZip(t, cbz, map[string]string{"01.jpg": "", "ComicInfo.xml": `<ComicInfo><GTIN>978-0-06-294150-3</GTIN></ComicInfo>`})
	if md, err := Read(cbz, "cbz"); err != nil || md.ISBN != "9780062941503" {
		t.Errorf("cbz: %+v, %v", md, err)
	}
}
