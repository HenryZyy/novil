package metadata

import (
	"errors"
	"path/filepath"
	"testing"

	"novil/backend/internal/testutil"
)

func writeEPUBWithCover(t *testing.T, path, metadata, manifest string, files map[string]string) {
	t.Helper()
	all := map[string]string{
		"mimetype": "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata>` + metadata + `</metadata>
  <manifest>` + manifest + `</manifest>
</package>`,
	}
	for k, v := range files {
		all[k] = v
	}
	testutil.WriteZip(t, path, all)
}

func TestEPUBCover(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, metadata, manifest string
		want                     string
	}{
		{"EPUB 3 property", ``, `<item id="a" href="img/a.jpg" media-type="image/jpeg" properties="cover-image"/>`, "jpeg-a"},
		{"EPUB 2 meta", `<meta name="cover" content="pic"/>`, `<item id="pic" href="img/b.png" media-type="image/png"/>`, "png-b"},
		{"file name", ``, `<item id="x" href="img/front-cover.jpg" media-type="image/jpeg"/>`, "jpeg-cover"},
	}
	files := map[string]string{"OEBPS/img/a.jpg": "jpeg-a", "OEBPS/img/b.png": "png-b", "OEBPS/img/front-cover.jpg": "jpeg-cover"}
	for i, tc := range cases {
		path := filepath.Join(dir, tc.name+".epub")
		writeEPUBWithCover(t, path, tc.metadata, tc.manifest, files)
		data, _, err := EPUBCover(path)
		if err != nil || string(data) != tc.want {
			t.Errorf("%d %s: got %q, %v; want %q", i, tc.name, data, err, tc.want)
		}
	}

	path := filepath.Join(dir, "none.epub")
	writeEPUBWithCover(t, path, ``, `<item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>`, nil)
	if _, _, err := EPUBCover(path); !errors.Is(err, ErrNoCover) {
		t.Errorf("no cover: got %v, want ErrNoCover", err)
	}
}
