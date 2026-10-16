package api

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"novil/backend/internal/store"
	"novil/backend/internal/testutil"
)

// getCover fetches a book's cover and returns the response and its body.
func getCover(c *client, bookID string, header ...string) (*http.Response, []byte) {
	c.t.Helper()
	req, _ := http.NewRequest("GET", c.srv.URL+"/api/books/"+bookID+"/cover", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

// centerColor decodes a JPEG and returns the color in its middle.
func centerColor(t *testing.T, data []byte) color.RGBA {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("cover is not a JPEG: %v", err)
	}
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
}

func isRed(c color.RGBA) bool  { return c.R > 200 && c.G < 60 && c.B < 60 }
func isBlue(c color.RGBA) bool { return c.B > 200 && c.R < 60 && c.G < 60 }

func TestCovers(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	root := t.TempDir()
	red := string(testutil.PNG(t, 40, 60, color.RGBA{255, 0, 0, 255}))
	testutil.WriteZip(t, filepath.Join(root, "Comic", "1.cbz"), map[string]string{"01.png": red})
	testutil.WriteFile(t, filepath.Join(root, "Docs", "report.pdf"), testutil.PDF())
	testutil.WriteEPUB(t, filepath.Join(root, "Novel", "plain.epub"), `<dc:title>No pictures</dc:title>`)
	admin.do("POST", "/api/libraries", libraryJSON(t, "Lib", root), http.StatusCreated, nil)
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	// Series and books sort by name: Comic (1), Docs (2), Novel (3).

	// A CBZ's first page and a PDF's first page become covers.
	for _, id := range []string{"1", "2"} {
		resp, data := getCover(admin, id)
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" || !isRed(centerColor(t, data)) {
			t.Fatalf("book %s: status %d, type %s", id, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	// An EPUB without images has none.
	if resp, _ := getCover(admin, "3"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("epub without images: status %d, want 404", resp.StatusCode)
	}

	// Series point at a book for their cover, whatever its format.
	var series []store.Series
	admin.do("GET", "/api/series", "", http.StatusOK, &series)
	for _, sr := range series {
		if sr.CoverBookID == nil {
			t.Errorf("series %s has no cover book", sr.Name)
		}
	}

	// Unchanged covers cost a 304.
	resp, _ := getCover(admin, "1")
	etag := resp.Header.Get("ETag")
	if resp, _ := getCover(admin, "1", "If-None-Match", etag); resp.StatusCode != http.StatusNotModified {
		t.Errorf("same ETag: status %d, want 304", resp.StatusCode)
	}

	// An admin replaces the cover.
	blue := testutil.PNG(t, 40, 60, color.RGBA{0, 0, 255, 255})
	var b store.Book
	admin.do("PUT", "/api/books/1/cover", string(blue), http.StatusOK, &b)
	if !b.CustomCover {
		t.Error("customCover is false after upload")
	}
	resp, data := getCover(admin, "1", "If-None-Match", etag)
	if resp.StatusCode != http.StatusOK || !isBlue(centerColor(t, data)) {
		t.Errorf("after upload: status %d, want the new blue cover", resp.StatusCode)
	}
	// It also gives the EPUB one.
	admin.do("PUT", "/api/books/3/cover", string(blue), http.StatusOK, nil)
	if resp, _ := getCover(admin, "3"); resp.StatusCode != http.StatusOK {
		t.Errorf("epub with uploaded cover: status %d", resp.StatusCode)
	}

	// Only images are accepted.
	admin.do("PUT", "/api/books/1/cover", "MZ\x90\x00 not an image", http.StatusBadRequest, nil)
	admin.do("PUT", "/api/books/1/cover", testutil.PDF(), http.StatusBadRequest, nil)
	admin.do("PUT", "/api/books/99/cover", string(blue), http.StatusNotFound, nil)

	// Deleting it goes back to the file's cover.
	admin.do("DELETE", "/api/books/1/cover", "", http.StatusOK, &b)
	if b.CustomCover {
		t.Error("customCover still true after delete")
	}
	if _, data := getCover(admin, "1"); !isRed(centerColor(t, data)) {
		t.Error("after delete: want the red first page again")
	}

	// Everyone sees covers; only admins change them.
	admin.do("POST", "/api/users", `{"username":"anna","password":"start-password"}`, http.StatusCreated, nil)
	anna := env.login(t, "anna", "start-password")
	var s session
	anna.do("PUT", "/api/me/password", `{"currentPassword":"start-password","newPassword":"anna-password"}`, http.StatusOK, &s)
	anna.token = s.Token
	if resp, _ := getCover(anna, "1"); resp.StatusCode != http.StatusOK {
		t.Errorf("user reading cover: status %d", resp.StatusCode)
	}
	anna.do("PUT", "/api/books/1/cover", string(blue), http.StatusForbidden, nil)
	anna.do("DELETE", "/api/books/1/cover", "", http.StatusForbidden, nil)
}

func TestCoverFollowsFileChanges(t *testing.T) {
	env := newTestServer(t)
	admin := env.admin
	root := t.TempDir()
	path := filepath.Join(root, "Comic", "1.cbz")
	testutil.WriteZip(t, path, map[string]string{"01.png": string(testutil.PNG(t, 40, 60, color.RGBA{255, 0, 0, 255}))})
	admin.do("POST", "/api/libraries", libraryJSON(t, "Lib", root), http.StatusCreated, nil)
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	if _, data := getCover(admin, "1"); !isRed(centerColor(t, data)) {
		t.Fatal("first cover not red")
	}

	// The file is replaced; after the next scan the cover is made again.
	testutil.WriteZip(t, path, map[string]string{"01.png": string(testutil.PNG(t, 50, 70, color.RGBA{0, 0, 255, 255}))})
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	admin.do("POST", "/api/libraries/1/scan", "", http.StatusOK, nil)
	if _, data := getCover(admin, "1"); !isBlue(centerColor(t, data)) {
		t.Error("cover didn't follow the changed file")
	}
}
