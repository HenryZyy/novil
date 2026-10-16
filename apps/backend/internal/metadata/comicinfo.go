package metadata

import (
	"archive/zip"
	"fmt"
	"strings"
)

// Many comic archives carry a ComicInfo.xml, the format of the ComicRack app
// that other comic tools adopted.
func readComicInfo(path string) (Metadata, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Metadata{}, err
	}
	defer zr.Close()

	name := ""
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, "ComicInfo.xml") {
			name = f.Name
			break
		}
	}
	if name == "" {
		return Metadata{}, nil
	}
	var ci struct {
		Title       string `xml:"Title"`
		Writer      string `xml:"Writer"`
		LanguageISO string `xml:"LanguageISO"`
		Year        int    `xml:"Year"`
		Month       int    `xml:"Month"`
		Day         int    `xml:"Day"`
		Summary     string `xml:"Summary"`
	}
	if err := decodeXML(&zr.Reader, name, &ci); err != nil {
		return Metadata{}, err
	}

	md := Metadata{
		Title:       clean(ci.Title),
		Author:      clean(ci.Writer),
		Language:    clean(ci.LanguageISO),
		Description: plainText(ci.Summary),
	}
	switch {
	case ci.Year > 0 && ci.Month > 0 && ci.Day > 0:
		md.Released = fmt.Sprintf("%04d-%02d-%02d", ci.Year, ci.Month, ci.Day)
	case ci.Year > 0 && ci.Month > 0:
		md.Released = fmt.Sprintf("%04d-%02d", ci.Year, ci.Month)
	case ci.Year > 0:
		md.Released = fmt.Sprintf("%04d", ci.Year)
	}
	return md, nil
}
