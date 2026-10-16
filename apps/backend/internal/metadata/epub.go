package metadata

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// An EPUB is a zip. META-INF/container.xml points to the package document
// (.opf), whose <metadata> holds Dublin Core fields like dc:title.
func readEPUB(path string) (Metadata, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Metadata{}, err
	}
	defer zr.Close()

	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeXML(&zr.Reader, "META-INF/container.xml", &container); err != nil {
		return Metadata{}, err
	}
	if len(container.Rootfiles) == 0 {
		return Metadata{}, errors.New("container.xml names no package document")
	}

	var opf struct {
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
			Metas        []struct {
				Refines  string `xml:"refines,attr"`
				Property string `xml:"property,attr"`
				Value    string `xml:",chardata"`
			} `xml:"meta"`
		} `xml:"metadata"`
	}
	if err := decodeXML(&zr.Reader, container.Rootfiles[0].FullPath, &opf); err != nil {
		return Metadata{}, err
	}
	m := opf.Metadata

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
		if err := xml.NewDecoder(io.LimitReader(rc, 10<<20)).Decode(v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	return fmt.Errorf("%s not found", name)
}
