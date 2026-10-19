// Package metadata reads the title, author and other details that book
// files carry about themselves.
package metadata

import (
	"html"
	"regexp"
	"strings"
)

// Metadata is what a file says about its book. Empty fields are unknown.
type Metadata struct {
	Title    string
	Author   string
	Language string
	// Released is the publication date as "2006", "2006-01" or "2006-01-02".
	Released    string
	Description string
	// ISBN is the ISBN-13, if the file names a valid one.
	ISBN string
}

// Read returns the metadata of a book file. Formats without readable
// metadata (PDF for now) return empty metadata.
func Read(path, format string) (Metadata, error) {
	switch format {
	case "epub":
		return readEPUB(path)
	case "cbz":
		return readComicInfo(path)
	}
	return Metadata{}, nil
}

var datePattern = regexp.MustCompile(`^(\d{4})(?:-(\d{2})(?:-(\d{2}))?)?`)

// NormalizeDate cuts a date like "2019-05-03T00:00:00Z" down to its date
// part. It returns "" for anything that doesn't start with a year.
func NormalizeDate(s string) string {
	return datePattern.FindString(strings.TrimSpace(s))
}

// ValidDate reports whether s is "", "2006", "2006-01" or "2006-01-02".
func ValidDate(s string) bool {
	return s == "" || NormalizeDate(s) == s
}

var (
	tagPattern   = regexp.MustCompile(`<[^>]*>`)
	spacePattern = regexp.MustCompile(`[ \t\r\f\v]+`)
	blankLines   = regexp.MustCompile(`\n\s*\n\s*`)
)

// plainText turns a description, which is often HTML, into plain text.
// Clients show it as text, so markup from a file can never run as code.
func plainText(s string) string {
	s = strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n\n").Replace(s)
	s = html.UnescapeString(tagPattern.ReplaceAllString(s, ""))
	s = spacePattern.ReplaceAllString(s, " ")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// clean trims a single-line value and collapses whitespace in it.
func clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
