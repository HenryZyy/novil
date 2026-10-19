package metadata

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"

	"novil/backend/internal/isbn"
)

// packageDoc is the part of an EPUB's package document (.opf) we use.
type packageDoc struct {
	Metadata struct {
		Titles   []string `xml:"title"`
		Creators []struct {
			ID   string `xml:"id,attr"`
			Role string `xml:"role,attr"` // EPUB 2: opf:role="aut"
			Name string `xml:",chardata"`
		} `xml:"creator"`
		Languages []string `xml:"language"`
		Dates     []struct {
			Event string `xml:"event,attr"` // EPUB 2: opf:event="publication"
			Value string `xml:",chardata"`
		} `xml:"date"`
		Descriptions []string `xml:"description"`
		Identifiers  []struct {
			Scheme string `xml:"scheme,attr"` // EPUB 2: opf:scheme="ISBN"
			Value  string `xml:",chardata"`
		} `xml:"identifier"`
		Metas []struct {
			Refines  string `xml:"refines,attr"`
			Property string `xml:"property,attr"`
			Name     string `xml:"name,attr"`    // EPUB 2: <meta name="cover" content="id">
			Content  string `xml:"content,attr"` //
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

// readPackage finds and parses the package document. An EPUB is a zip, and
// META-INF/container.xml says where in it the .opf is. It also returns the
// .opf's folder, which its links are relative to.
func readPackage(zr *zip.Reader) (packageDoc, string, error) {
	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeXML(zr, "META-INF/container.xml", &container); err != nil {
		return packageDoc{}, "", err
	}
	if len(container.Rootfiles) == 0 {
		return packageDoc{}, "", errors.New("container.xml names no package document")
	}
	var pkg packageDoc
	opfPath := container.Rootfiles[0].FullPath
	if err := decodeXML(zr, opfPath, &pkg); err != nil {
		return packageDoc{}, "", err
	}
	return pkg, path.Dir(opfPath), nil
}

func readEPUB(file string) (Metadata, error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return Metadata{}, err
	}
	defer zr.Close()
	pkg, _, err := readPackage(&zr.Reader)
	if err != nil {
		return Metadata{}, err
	}
	m := pkg.Metadata

	var md Metadata
	if len(m.Titles) > 0 {
		md.Title = clean(m.Titles[0])
	}
	if len(m.Languages) > 0 {
		md.Language = clean(m.Languages[0])
	}
	if len(m.Descriptions) > 0 {
		md.Description = plainText(m.Descriptions[0])
	}
	// Identifiers are often UUIDs; an ISBN is one whose check digit works,
	// written like "urn:isbn:978…" or plain.
	for _, id := range m.Identifiers {
		v := strings.TrimSpace(id.Value)
		for _, prefix := range []string{"urn:isbn:", "isbn:"} {
			if len(v) > len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
				v = v[len(prefix):]
			}
		}
		if code, err := isbn.Normalize(v); err == nil {
			md.ISBN = code
			break
		}
	}

	// Creators can be authors, illustrators, editors... EPUB 2 says which in
	// an attribute, EPUB 3 in a <meta refines="#id" property="role">.
	roles := map[string]string{}
	for _, meta := range m.Metas {
		if meta.Property == "role" {
			roles[strings.TrimPrefix(meta.Refines, "#")] = clean(meta.Value)
		}
	}
	var authors, all []string
	for _, c := range m.Creators {
		name := clean(c.Name)
		if name == "" {
			continue
		}
		all = append(all, name)
		role := c.Role
		if role == "" {
			role = roles[c.ID]
		}
		if role == "aut" {
			authors = append(authors, name)
		}
	}
	if len(authors) == 0 {
		authors = all
	}
	md.Author = strings.Join(authors, ", ")

	// Prefer the publication date; EPUB 2 files may also list others.
	for _, d := range m.Dates {
		if date := NormalizeDate(d.Value); date != "" && (md.Released == "" || d.Event == "publication") {
			md.Released = date
		}
	}
	return md, nil
}

var ErrNoCover = errors.New("no cover image")

// EPUBCover returns the name of the cover image inside an EPUB's zip. It
// tries, in order: the image EPUB 3 marks as cover, the one EPUB 2 names in
// <meta name="cover">, and the first image on the book's first pages.
func EPUBCover(zr *zip.Reader) (string, error) {
	pkg, dir, err := readPackage(zr)
	if err != nil {
		return "", err
	}
	byID := map[string]string{}
	for _, item := range pkg.Manifest {
		byID[item.ID] = item.Href
		if strings.Contains(" "+item.Properties+" ", " cover-image ") {
			return resolve(dir, item.Href), nil
		}
	}
	for _, meta := range pkg.Metadata.Metas {
		if meta.Name == "cover" && byID[meta.Content] != "" {
			return resolve(dir, byID[meta.Content]), nil
		}
	}
	// Often the first page is a cover page with just the image on it.
	for i, ref := range pkg.Spine {
		if i == 3 {
			break
		}
		page := resolve(dir, byID[ref.IDRef])
		if img := firstImage(zr, page); img != "" {
			return img, nil
		}
	}
	return "", ErrNoCover
}

var imageRef = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["']([^"'#?]+\.(?:jpe?g|png|gif|webp))["']`)

// firstImage returns the first image a (X)HTML page in the zip links to.
func firstImage(zr *zip.Reader, page string) string {
	f, err := zr.Open(page)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return ""
	}
	m := imageRef.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return resolve(path.Dir(page), string(m[1]))
}

// resolve turns a link relative to dir into a name in the zip.
func resolve(dir, href string) string {
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	return path.Join(dir, href)
}

// decodeXML reads the named file in the zip as XML into v.
func decodeXML(zr *zip.Reader, name string, v any) error {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		// Metadata files are small; the limit guards against zip bombs.
		d := xml.NewDecoder(io.LimitReader(rc, 10<<20))
		// Some tools write HTML's &rsquo; and the like, which XML lacks.
		d.Entity = xml.HTMLEntity
		if err := d.Decode(v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	return fmt.Errorf("%s not found", name)
}
