package cover

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"novil/backend/internal/testutil"
)

var (
	red  = color.RGBA{255, 0, 0, 255}
	blue = color.RGBA{0, 0, 255, 255}
)

// decodeJPEG checks that data is a JPEG and returns its size and the color
// in its middle.
func decodeJPEG(t *testing.T, data []byte) (int, int, color.RGBA) {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	return b.Dx(), b.Dy(), color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
}

// near reports whether two colors are about the same; JPEG is lossy.
func near(a, b color.RGBA) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) < 20 && d(a.G, b.G) < 20 && d(a.B, b.B) < 20
}

func TestCBZCover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.cbz")
	testutil.WriteZip(t, path, map[string]string{
		"02.png": string(testutil.PNG(t, 10, 10, blue)),
		"01.png": string(testutil.PNG(t, 1000, 3000, red)),
	})
	data, err := NewGenerator().FromBook(path, "cbz")
	if err != nil {
		t.Fatal(err)
	}
	w, h, c := decodeJPEG(t, data)
	// 1000x3000 scaled to fit 800x1200 keeps its 1:3 shape.
	if w != 400 || h != 1200 || !near(c, red) {
		t.Errorf("cover %dx%d %v, want 400x1200 red (page 01)", w, h, c)
	}
}

func TestEPUBCover(t *testing.T) {
	container := `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`
	opf := func(metadata, manifest, spine string) string {
		return `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf"><metadata>` + metadata +
			`</metadata><manifest>` + manifest + `</manifest><spine>` + spine + `</spine></package>`
	}
	redPNG, bluePNG := string(testutil.PNG(t, 20, 30, red)), string(testutil.PNG(t, 20, 30, blue))

	tests := []struct {
		name  string
		files map[string]string
	}{
		{"epub 3 cover-image", map[string]string{
			"OEBPS/content.opf":       opf("", `<item id="x" href="images/c%20over.png" media-type="image/png" properties="cover-image"/>`, ""),
			"OEBPS/images/c over.png": redPNG,
		}},
		{"epub 2 meta cover", map[string]string{
			"OEBPS/content.opf": opf(`<meta name="cover" content="img1"/>`,
				`<item id="other" href="other.png" media-type="image/png"/><item id="img1" href="../cover.png" media-type="image/png"/>`, ""),
			"cover.png":       redPNG,
			"OEBPS/other.png": bluePNG,
		}},
		{"image on first page", map[string]string{
			"OEBPS/content.opf":      opf("", `<item id="p1" href="text/title.xhtml" media-type="application/xhtml+xml"/>`, `<itemref idref="p1"/>`),
			"OEBPS/text/title.xhtml": `<html><body><svg><image xlink:href="../img/front.png"/></svg></body></html>`,
			"OEBPS/img/front.png":    redPNG,
		}},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "book.epub")
		tt.files["mimetype"] = "application/epub+zip"
		tt.files["META-INF/container.xml"] = container
		testutil.WriteZip(t, path, tt.files)
		data, err := NewGenerator().FromBook(path, "epub")
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if _, _, c := decodeJPEG(t, data); !near(c, red) {
			t.Errorf("%s: cover color %v, want red", tt.name, c)
		}
	}

	path := filepath.Join(t.TempDir(), "none.epub")
	testutil.WriteEPUB(t, path, `<dc:title>No pictures</dc:title>`)
	if _, err := NewGenerator().FromBook(path, "epub"); !errors.Is(err, ErrNoCover) {
		t.Errorf("epub without images: %v, want ErrNoCover", err)
	}
}

func TestPDFCover(t *testing.T) {
	if testing.Short() {
		t.Skip("starting PDFium takes a few seconds")
	}
	path := filepath.Join(t.TempDir(), "book.pdf")
	testutil.WriteFile(t, path, testutil.PDF())
	g := NewGenerator()
	defer g.Close()
	data, err := g.FromBook(path, "pdf")
	if err != nil {
		t.Fatal(err)
	}
	// A 200x300 page rendered to fit 800x1200.
	if w, h, c := decodeJPEG(t, data); w != 800 || h != 1200 || !near(c, red) {
		t.Errorf("cover %dx%d %v, want 800x1200 red", w, h, c)
	}

	broken := filepath.Join(t.TempDir(), "broken.pdf")
	testutil.WriteFile(t, broken, "%PDF-1.4\nnot really")
	if _, err := g.FromBook(broken, "pdf"); err == nil {
		t.Error("broken pdf: got nil error")
	}
}

func TestFromUpload(t *testing.T) {
	// Transparent parts turn white.
	transparent := testutil.PNG(t, 50, 50, color.RGBA{})
	data, err := FromUpload(bytes.NewReader(transparent))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, c := decodeJPEG(t, data); !near(c, color.RGBA{255, 255, 255, 255}) {
		t.Errorf("transparent became %v, want white", c)
	}

	for name, body := range map[string]string{
		"exe":  "MZ\x90\x00 not an image",
		"text": strings.Repeat("hello ", 100),
		"pdf":  testutil.PDF(),
	} {
		if _, err := FromUpload(strings.NewReader(body)); !errors.Is(err, ErrNotAnImage) {
			t.Errorf("%s: %v, want ErrNotAnImage", name, err)
		}
	}
}

func TestAvatarFromUpload(t *testing.T) {
	// 300x100: blue | red | blue. The square from the middle is all red.
	img := image.NewRGBA(image.Rect(0, 0, 300, 100))
	draw.Draw(img, img.Bounds(), image.NewUniform(blue), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(100, 0, 200, 100), image.NewUniform(red), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data, err := AvatarFromUpload(&buf)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 100 || b.Dy() != 100 {
		t.Errorf("avatar is %dx%d, want the 100x100 middle", b.Dx(), b.Dy())
	}
	for _, p := range []image.Point{{5, 50}, {50, 50}, {94, 50}} {
		r, g, b, _ := out.At(p.X, p.Y).RGBA()
		if c := (color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}); !near(c, red) {
			t.Errorf("pixel %v is %v, want red", p, c)
		}
	}

	// Large pictures shrink to AvatarSize.
	data, err = AvatarFromUpload(bytes.NewReader(testutil.PNG(t, 2000, 1500, red)))
	if err != nil {
		t.Fatal(err)
	}
	if w, h, _ := decodeJPEG(t, data); w != AvatarSize || h != AvatarSize {
		t.Errorf("avatar is %dx%d, want %d square", w, h, AvatarSize)
	}

	if _, err := AvatarFromUpload(strings.NewReader("MZ not an image")); !errors.Is(err, ErrNotAnImage) {
		t.Errorf("exe: %v, want ErrNotAnImage", err)
	}
}

func TestDecodeRefusesHugeImages(t *testing.T) {
	// A valid PNG header claiming 100000 x 100000 pixels. The header chunk
	// (IHDR) starts at byte 8: length, type, width, height, ..., checksum.
	huge := testutil.PNG(t, 1, 1, red)
	copy(huge[16:24], []byte{0, 1, 0x86, 0xa0, 0, 1, 0x86, 0xa0})
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, err := FromUpload(bytes.NewReader(huge)); err == nil || errors.Is(err, ErrNotAnImage) {
		t.Errorf("huge image: %v, want a too large error", err)
	}
}

func TestFit(t *testing.T) {
	for _, tt := range []struct{ w, h, ww, wh int }{
		{100, 100, 100, 100},
		{1600, 1200, 800, 600},
		{800, 2400, 400, 1200},
		{8000, 1, 800, 1},
	} {
		if w, h := fit(tt.w, tt.h, MaxWidth, MaxHeight); w != tt.ww || h != tt.wh {
			t.Errorf("fit(%d, %d) = %d, %d, want %d, %d", tt.w, tt.h, w, h, tt.ww, tt.wh)
		}
	}
}
