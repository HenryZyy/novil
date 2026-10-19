package isbn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
)

// ErrNotFound means no source knows the ISBN.
var ErrNotFound = errors.New("no book found for this ISBN")

type Info struct {
	ISBN    string `json:"isbn"`
	Title   string `json:"title"`
	Authors string `json:"authors"`
	// Published is the year, or the date as the source writes it.
	Published string `json:"published"`
	CoverURL  string `json:"coverUrl"`
	Pages     int    `json:"pages"`
	// Description comes from Google Books only.
	Description string `json:"description"`
}

// Lookup is a function that finds the book of an ISBN-13.
type Lookup func(ctx context.Context, isbn string) (Info, error)

var client = &http.Client{Timeout: 8 * time.Second}

// Online returns a Lookup that asks Open Library and Google Books at once.
// Google's titles and authors tend to be cleaner, Open Library's covers
// bigger, so each fills in what the other lacks. Open Library needs no
// key; Google works without one, but then shares a small daily quota with
// everyone else, so googleKey makes it reliable.
func Online(googleKey string) Lookup {
	return func(ctx context.Context, isbn string) (Info, error) {
		return online(ctx, isbn, googleKey)
	}
}

func online(ctx context.Context, isbn, googleKey string) (Info, error) {
	type result struct {
		info Info
		err  error
	}
	ol, gb := make(chan result, 1), make(chan result, 1)
	go func() { info, err := openLibrary(ctx, isbn); ol <- result{info, err} }()
	go func() { info, err := googleBooks(ctx, isbn, googleKey); gb <- result{info, err} }()
	o, g := <-ol, <-gb
	switch {
	case o.err != nil && g.err != nil:
		if errors.Is(o.err, ErrNotFound) && errors.Is(g.err, ErrNotFound) {
			return Info{}, ErrNotFound
		}
		return Info{}, errors.Join(o.err, g.err)
	case o.err != nil:
		return g.info, nil
	case g.err != nil:
		return o.info, nil
	}
	return Info{
		ISBN:        isbn,
		Title:       firstNonEmpty(g.info.Title, o.info.Title),
		Authors:     firstNonEmpty(g.info.Authors, o.info.Authors),
		Published:   firstNonEmpty(g.info.Published, o.info.Published),
		CoverURL:    firstNonEmpty(o.info.CoverURL, g.info.CoverURL),
		Pages:       max(g.info.Pages, o.info.Pages),
		Description: g.info.Description,
	}, nil
}

func getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "novil (self-hosted book server)")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func openLibrary(ctx context.Context, isbn string) (Info, error) {
	var data map[string]struct {
		Title    string `json:"title"`
		Subtitle string `json:"subtitle"`
		Authors  []struct {
			Name string `json:"name"`
		} `json:"authors"`
		PublishDate string `json:"publish_date"`
		Pages       int    `json:"number_of_pages"`
		Cover       struct {
			Large  string `json:"large"`
			Medium string `json:"medium"`
		} `json:"cover"`
	}
	u := "https://openlibrary.org/api/books?" + url.Values{"bibkeys": {"ISBN:" + isbn}, "format": {"json"}, "jscmd": {"data"}}.Encode()
	if err := getJSON(ctx, u, &data); err != nil {
		return Info{}, err
	}
	b, ok := data["ISBN:"+isbn]
	if !ok || b.Title == "" {
		return Info{}, ErrNotFound
	}
	names := []string{}
	for _, a := range b.Authors {
		names = addAuthor(names, a.Name)
	}
	title := b.Title
	if b.Subtitle != "" {
		title += ": " + b.Subtitle
	}
	return Info{
		ISBN: isbn, Title: title, Authors: strings.Join(names, ", "),
		Published: b.PublishDate, CoverURL: firstNonEmpty(b.Cover.Large, b.Cover.Medium), Pages: b.Pages,
	}, nil
}

func googleBooks(ctx context.Context, isbn, key string) (Info, error) {
	var data struct {
		Items []struct {
			VolumeInfo struct {
				Title         string   `json:"title"`
				Subtitle      string   `json:"subtitle"`
				Authors       []string `json:"authors"`
				PublishedDate string   `json:"publishedDate"`
				PageCount     int      `json:"pageCount"`
				Description   string   `json:"description"`
				ImageLinks    struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	query := url.Values{"q": {"isbn:" + isbn}}
	if key != "" {
		query.Set("key", key)
	}
	if err := getJSON(ctx, "https://www.googleapis.com/books/v1/volumes?"+query.Encode(), &data); err != nil {
		return Info{}, err
	}
	if len(data.Items) == 0 || data.Items[0].VolumeInfo.Title == "" {
		return Info{}, ErrNotFound
	}
	v := data.Items[0].VolumeInfo
	title := v.Title
	if v.Subtitle != "" {
		title += ": " + v.Subtitle
	}
	return Info{
		ISBN: isbn, Title: title, Authors: strings.Join(v.Authors, ", "), Published: v.PublishedDate,
		CoverURL: strings.Replace(v.ImageLinks.Thumbnail, "http://", "https://", 1), Pages: v.PageCount,
		Description: v.Description,
	}, nil
}

// addAuthor adds name unless it repeats one already there, which Open
// Library often has, cut short or reordered: "Russell, Kate Elizab".
func addAuthor(names []string, name string) []string {
	words := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) })
	if len(words) == 0 {
		return names
	}
	for _, other := range names {
		otherWords := strings.FieldsFunc(strings.ToLower(other), func(r rune) bool { return !unicode.IsLetter(r) })
		if slices.ContainsFunc(words, func(w string) bool {
			return !slices.ContainsFunc(otherWords, func(o string) bool { return strings.HasPrefix(o, w) })
		}) {
			continue
		}
		return names
	}
	return append(names, strings.Join(strings.Fields(name), " "))
}

// Fetcher downloads a cover image a Lookup found.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// maxCoverBytes keeps a misbehaving server from sending endless data.
const maxCoverBytes = 20 << 20

// FetchCover downloads a cover from the HTTPS URL a lookup returned.
func FetchCover(ctx context.Context, u string) ([]byte, error) {
	if !strings.HasPrefix(u, "https://") {
		return nil, fmt.Errorf("cover URL %q isn't HTTPS", u)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "novil (self-hosted book server)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
