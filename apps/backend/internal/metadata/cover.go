package metadata

import (
	"archive/zip"
	"errors"
	"io"
	"path"
	"strings"
)

// ErrNoCover is returned for books that don't contain a cover image.
var ErrNoCover = errors.New("no cover image")

// maxCoverBytes guards against huge or malicious images.
const maxCoverBytes = 20 << 20

// EPUBCover returns the cover image of an EPUB and its media type. Books
// mark it in different ways, so this tries, in order: the EPUB 3
// "cover-image" property, the EPUB 2 <meta name="cover">, and an image
// whose id or file name contains "cover".
func EPUBCover(epubPath string) ([]byte, string, error) {
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, "", err
	}
	defer zr.Close()

	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeXML(&zr.Reader, "META-INF/container.xml", &container); err != nil {
		return nil, "", err
	}
	if len(container.Rootfiles) == 0 {
		return nil, "", ErrNoCover
	}
	opfPath := container.Rootfiles[0].FullPath

	var opf struct {
		Metas []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"metadata>meta"`
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"manifest>item"`
	}
	if err := decodeXML(&zr.Reader, opfPath, &opf); err != nil {
		return nil, "", err
	}

	coverID := ""
	for _, m := range opf.Metas {
		if m.Name == "cover" {
			coverID = m.Content
		}
	}
	isImage := func(mediaType string) bool { return strings.HasPrefix(mediaType, "image/") }
	var href, mediaType string
	for _, pick := range []func(id, href, props string) bool{
		func(_, _, props string) bool { return strings.Contains(" "+props+" ", " cover-image ") },
		func(id, _, _ string) bool { return coverID != "" && id == coverID },
		func(id, href, _ string) bool {
			return strings.Contains(strings.ToLower(id), "cover") || strings.Contains(strings.ToLower(href), "cover")
		},
	} {
		for _, item := range opf.Items {
			if isImage(item.MediaType) && pick(item.ID, item.Href, item.Properties) {
				href, mediaType = item.Href, item.MediaType
				break
			}
		}
		if href != "" {
			break
		}
	}
	if href == "" {
		return nil, "", ErrNoCover
	}

	// Hrefs are relative to the package document.
	name := path.Join(path.Dir(opfPath), href)
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, maxCoverBytes+1))
		if err != nil {
			return nil, "", err
		}
		if len(data) > maxCoverBytes {
			return nil, "", errors.New("cover image is too large")
		}
		return data, mediaType, nil
	}
	return nil, "", ErrNoCover
}
