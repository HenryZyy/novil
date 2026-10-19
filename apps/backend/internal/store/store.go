// Package store reads and writes the database.
package store

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"novil/backend/internal/scanner"
)

var ErrNotFound = errors.New("not found")

// Fixed width, so the stored strings sort in time order.
const timeMillis = "2006-01-02T15:04:05.000Z07:00"

type Library struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	AllowUploads bool   `json:"allowUploads"`
}

type Series struct {
	ID           int64  `json:"id"`
	LibraryID    int64  `json:"libraryId"`
	Name         string `json:"name"`
	BookCount    int    `json:"bookCount"`
	CoverBookID  *int64 `json:"coverBookId"`
	ReadCount    int    `json:"readCount"`
	ReadingCount int    `json:"readingCount"`
	// HiddenCount is how many books only admins see. Always 0 for users,
	// who don't get hidden books at all.
	HiddenCount int `json:"hiddenCount"`
	// Progress is how much of the series was read, from 0 to 1.
	Progress   float64 `json:"progress"`
	AddedAt    string  `json:"addedAt"`
	LastReadAt *string `json:"lastReadAt"`
	// The user's lists the series is on.
	Favorite   bool `json:"favorite"`
	WantToRead bool `json:"wantToRead"`
}

type Book struct {
	ID         int64  `json:"id"`
	SeriesID   int64  `json:"seriesId"`
	SeriesName string `json:"seriesName"`
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	PageCount  int    `json:"pageCount"`
	// AddedAt is when the book was uploaded or first found by a scan.
	AddedAt string `json:"addedAt"`

	// Metadata from the file, or what an admin set instead. Edited lists
	// the fields an admin set.
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Language    string   `json:"language"`
	Released    string   `json:"released"`
	Description string   `json:"description"`
	ISBN        string   `json:"isbn"`
	Edited      []string `json:"edited"`

	// CustomCover is true when an admin uploaded the cover.
	CustomCover bool `json:"customCover"`
	// Hidden books are only shown to admins.
	Hidden bool `json:"hidden"`

	ReadPage     int    `json:"readPage"`
	ReadPosition string `json:"readPosition"`
	Completed    bool   `json:"completed"`
	Path         string `json:"-"`
	// FileKey changes when the file does; see CachedCover.
	FileKey string `json:"-"`
}

// MetadataFields are the book fields an admin can edit, named as in JSON
// and in the books and book_overrides tables.
var MetadataFields = []string{"title", "author", "language", "released", "description", "isbn"}

type Progress struct {
	BookID int64 `json:"bookId"`
	// Page is the page for comics, and the percentage read for EPUBs.
	Page int `json:"page"`
	// Position is the EPUB CFI of the reading position; empty for comics.
	Position  string    `json:"position"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) CreateLibrary(name, path string) (Library, error) {
	res, err := s.db.Exec(`INSERT INTO libraries (name, path) VALUES (?, ?)`, name, path)
	if err != nil {
		return Library{}, err
	}
	id, err := res.LastInsertId()
	return Library{ID: id, Name: name, Path: path}, err
}

const librarySelect = `SELECT id, name, path, allow_uploads FROM libraries`

func (s *Store) ListLibraries() ([]Library, error) {
	rows, err := s.db.Query(librarySelect + ` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Library{}
	for rows.Next() {
		var l Library
		if err := rows.Scan(&l.ID, &l.Name, &l.Path, &l.AllowUploads); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) GetLibrary(id int64) (Library, error) {
	var l Library
	err := s.db.QueryRow(librarySelect+` WHERE id = ?`, id).Scan(&l.ID, &l.Name, &l.Path, &l.AllowUploads)
	return l, notFound(err)
}

func (s *Store) SetAllowUploads(id int64, allow bool) (Library, error) {
	res, err := s.db.Exec(`UPDATE libraries SET allow_uploads = ? WHERE id = ?`, allow, id)
	if err != nil {
		return Library{}, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return Library{}, cmp.Or(err, ErrNotFound)
	}
	return s.GetLibrary(id)
}

// visibleTo is true for the books (named by alias) that user ?1 may see:
// all of them for admins, the ones not hidden for everyone else. Each
// request rereads the role, so a role change applies right away.
func visibleTo(alias string) string {
	return `(NOT ` + alias + `.hidden OR EXISTS (SELECT 1 FROM users WHERE id = ?1 AND role = 'admin'))`
}

// Series without a visible book don't exist for the user; see the HAVING
// in ListSeries and GetSeries.
var seriesSelect = `
	SELECT s.id, s.library_id, s.name, COUNT(b.id),
		(SELECT id FROM books cb WHERE cb.series_id = s.id AND ` + visibleTo("cb") + ` ORDER BY cb.sort_order LIMIT 1),
		COUNT(CASE WHEN p.completed THEN 1 END),
		COUNT(CASE WHEN p.page > 0 AND NOT p.completed THEN 1 END),
		COUNT(CASE WHEN b.hidden THEN 1 END),
		-- How much of the series was read: each book's share, a page out of
		-- its pages, or for EPUBs (no pages) the percentage they store.
		COALESCE(SUM(CASE
			WHEN p.completed THEN 1.0
			WHEN b.page_count > 0 THEN MIN(COALESCE(p.page, 0) * 1.0 / b.page_count, 1.0)
			ELSE MIN(COALESCE(p.page, 0) / 100.0, 1.0)
		END) / COUNT(b.id), 0),
		COALESCE(MAX(b.created_at), ''), MAX(p.updated_at),
		EXISTS (SELECT 1 FROM series_lists WHERE user_id = ?1 AND series_id = s.id AND list = 'favorites'),
		EXISTS (SELECT 1 FROM series_lists WHERE user_id = ?1 AND series_id = s.id AND list = 'want')
	FROM series s
	LEFT JOIN books b ON b.series_id = s.id AND ` + visibleTo("b") + `
	LEFT JOIN progress p ON p.book_id = b.id AND p.user_id = ?1
	LEFT JOIN book_overrides o ON o.book_id = b.id`

func scanSeries(row interface{ Scan(...any) error }) (Series, error) {
	var sr Series
	err := row.Scan(&sr.ID, &sr.LibraryID, &sr.Name, &sr.BookCount, &sr.CoverBookID,
		&sr.ReadCount, &sr.ReadingCount, &sr.HiddenCount, &sr.Progress, &sr.AddedAt, &sr.LastReadAt, &sr.Favorite, &sr.WantToRead)
	return sr, err
}

type SeriesQuery struct {
	LibraryID int64
	Search    string
	// Status is "unread", "reading", "read" or empty for all.
	Status string
	// List is "favorites" or "want" for only the series on that list of
	// the user, or empty for all.
	List string
	// Sort is one of seriesSort's keys; empty is by name.
	Sort string
}

var seriesStatus = map[string]string{
	"":        ``,
	"unread":  ` AND COUNT(p.book_id) = 0`,
	"reading": ` AND COUNT(p.book_id) > 0 AND COUNT(CASE WHEN p.completed THEN 1 END) < COUNT(b.id)`,
	"read":    ` AND COUNT(CASE WHEN p.completed THEN 1 END) = COUNT(b.id)`,
}

// A series' release date is its books' latest or earliest, counting admin
// edits. Series without one sort last either way.
const released = `NULLIF(COALESCE(o.released, b.released), '')`

var seriesSort = map[string]string{
	"":             `s.name COLLATE NOCASE`,
	"name-desc":    `s.name COLLATE NOCASE DESC`,
	"added":        `MAX(b.created_at) DESC, s.name COLLATE NOCASE`,
	"added-asc":    `MIN(b.created_at), s.name COLLATE NOCASE`,
	"released":     `MAX(` + released + `) IS NULL, MAX(` + released + `) DESC, s.name COLLATE NOCASE`,
	"released-asc": `MIN(` + released + `) IS NULL, MIN(` + released + `), s.name COLLATE NOCASE`,
	"read":         `MAX(p.updated_at) IS NULL, MAX(p.updated_at) DESC, s.name COLLATE NOCASE`,
	// Recently put on the list in SeriesQuery.List (?4).
	"listed": `(SELECT added_at FROM series_lists WHERE user_id = ?1 AND series_id = s.id AND list = ?4) DESC, s.name COLLATE NOCASE`,
}

// ValidSort reports whether sort is one ListSeries knows.
func ValidSort(sort string) bool {
	_, ok := seriesSort[sort]
	return ok
}

func (s *Store) ListSeries(userID int64, q SeriesQuery) ([]Series, error) {
	order, ok := seriesSort[q.Sort]
	if !ok {
		return nil, fmt.Errorf("unknown sort %q", q.Sort)
	}
	having, ok := seriesStatus[q.Status]
	if !ok {
		return nil, fmt.Errorf("unknown status %q", q.Status)
	}
	rows, err := s.db.Query(seriesSelect+`
		WHERE (?2 = 0 OR s.library_id = ?2) AND (?3 = '' OR instr(lower(s.name), lower(?3)) > 0)
		AND (?4 = '' OR EXISTS (SELECT 1 FROM series_lists WHERE user_id = ?1 AND series_id = s.id AND list = ?4))
		GROUP BY s.id HAVING COUNT(b.id) > 0`+having+` ORDER BY `+order, userID, q.LibraryID, q.Search, q.List)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Series{}
	for rows.Next() {
		sr, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

func (s *Store) GetSeries(userID, id int64) (Series, error) {
	sr, err := scanSeries(s.db.QueryRow(seriesSelect+` WHERE s.id = ?2 GROUP BY s.id HAVING COUNT(b.id) > 0`, userID, id))
	return sr, notFound(err)
}

// bookSelect reads each metadata field twice: from the file (books) and
// as set by an admin (book_overrides, NULL if not set), in MetadataFields
// order.
// Every query on it filters with visibleTo("b").
const bookSelect = `
	SELECT b.id, b.series_id, s.name, b.format, b.size, b.page_count, b.created_at,
		COALESCE(p.page, 0), COALESCE(p.position, ''), COALESCE(p.completed, 0), b.path,
		b.size || '-' || b.mod_time, EXISTS (SELECT 1 FROM book_covers WHERE book_id = b.id), b.hidden,
		b.title, o.title, b.author, o.author, b.language, o.language,
		b.released, o.released, b.description, o.description, b.isbn, o.isbn
	FROM books b
	JOIN series s ON s.id = b.series_id
	LEFT JOIN progress p ON p.book_id = b.id AND p.user_id = ?1
	LEFT JOIN book_overrides o ON o.book_id = b.id`

func scanBook(row interface{ Scan(...any) error }) (Book, error) {
	var b Book
	var fromFile [6]string
	var override [6]sql.NullString
	err := row.Scan(&b.ID, &b.SeriesID, &b.SeriesName, &b.Format, &b.Size, &b.PageCount, &b.AddedAt,
		&b.ReadPage, &b.ReadPosition, &b.Completed, &b.Path,
		&b.FileKey, &b.CustomCover, &b.Hidden,
		&fromFile[0], &override[0], &fromFile[1], &override[1], &fromFile[2], &override[2],
		&fromFile[3], &override[3], &fromFile[4], &override[4], &fromFile[5], &override[5])
	if err != nil {
		return b, err
	}
	b.Edited = []string{}
	for i, field := range []*string{&b.Title, &b.Author, &b.Language, &b.Released, &b.Description, &b.ISBN} {
		*field = fromFile[i]
		if override[i].Valid {
			*field = override[i].String
			b.Edited = append(b.Edited, MetadataFields[i])
		}
	}
	return b, nil
}

// SetMetadata changes the admin-set metadata of a book. A value replaces
// the file's; nil goes back to the file's. Fields not in changes stay as
// they are.
func (s *Store) SetMetadata(bookID int64, changes map[string]*string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO book_overrides (book_id) VALUES (?) ON CONFLICT DO NOTHING`, bookID); err != nil {
		return err
	}
	for _, field := range MetadataFields {
		v, ok := changes[field]
		if !ok {
			continue
		}
		// field comes from MetadataFields, never from the request.
		if _, err := tx.Exec(`UPDATE book_overrides SET `+field+` = ? WHERE book_id = ?`, v, bookID); err != nil {
			return err
		}
	}
	// Tidy up books that are back to their file's values.
	if _, err := tx.Exec(`
		DELETE FROM book_overrides WHERE book_id = ? AND title IS NULL AND author IS NULL
		AND language IS NULL AND released IS NULL AND description IS NULL AND isbn IS NULL`, bookID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) listBooks(query string, args ...any) ([]Book, error) {
	rows, err := s.db.Query(bookSelect+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) ListBooks(userID, seriesID int64) ([]Book, error) {
	return s.listBooks(` WHERE b.series_id = ?2 AND `+visibleTo("b")+` ORDER BY b.sort_order`, userID, seriesID)
}

type BookQuery struct {
	LibraryID int64
	// Search matches the title (an admin's or the file's), the author and
	// the series name.
	Search string
	// Hidden is "hidden", "visible" or empty for all.
	Hidden string
}

// SearchBooks lists the books of every series, by series and then in
// series order, for the admin dashboard.
func (s *Store) SearchBooks(userID int64, q BookQuery) ([]Book, error) {
	return s.listBooks(` WHERE `+visibleTo("b")+`
		AND (?2 = 0 OR s.library_id = ?2)
		AND (?3 = '' OR instr(lower(COALESCE(o.title, b.title) || ' ' || COALESCE(o.author, b.author) || ' ' || s.name), lower(?3)) > 0)
		AND (?4 = '' OR b.hidden = (?4 = 'hidden'))
		ORDER BY s.name COLLATE NOCASE, b.sort_order`, userID, q.LibraryID, q.Search, q.Hidden)
}

// ContinueReading returns the books the user has started but not finished.
func (s *Store) ContinueReading(userID int64, limit int) ([]Book, error) {
	return s.listBooks(` WHERE p.page > 0 AND NOT p.completed AND `+visibleTo("b")+` ORDER BY p.updated_at DESC LIMIT ?2`, userID, limit)
}

func (s *Store) GetBook(userID, id int64) (Book, error) {
	b, err := scanBook(s.db.QueryRow(bookSelect+` WHERE b.id = ?2 AND `+visibleTo("b"), userID, id))
	return b, notFound(err)
}

func (s *Store) GetBookByPath(userID int64, path string) (Book, error) {
	b, err := scanBook(s.db.QueryRow(bookSelect+` WHERE b.path = ?2 AND `+visibleTo("b"), userID, path))
	return b, notFound(err)
}

// GetProgress returns the user's progress in a book, or page 0 if they
// haven't opened it yet.
func (s *Store) GetProgress(userID, bookID int64) (Progress, error) {
	p := Progress{BookID: bookID}
	var updated string
	err := s.db.QueryRow(`SELECT page, position, completed, updated_at FROM progress WHERE user_id = ? AND book_id = ?`, userID, bookID).
		Scan(&p.Page, &p.Position, &p.Completed, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updated)
	return p, err
}

func (s *Store) SetProgress(userID int64, p Progress) (Progress, error) {
	p.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	_, err := s.db.Exec(`
		INSERT INTO progress (user_id, book_id, page, position, completed, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, book_id) DO UPDATE SET
			page = excluded.page, position = excluded.position, completed = excluded.completed,
			updated_at = excluded.updated_at`,
		userID, p.BookID, p.Page, p.Position, p.Completed, p.UpdatedAt.Format(timeMillis))
	return p, err
}

// SetHidden hides a book from users, or shows it again.
func (s *Store) SetHidden(bookID int64, hidden bool) error {
	_, err := s.db.Exec(`UPDATE books SET hidden = ? WHERE id = ?`, hidden, bookID)
	return err
}

// SetSeriesHidden hides or shows every book in a series.
func (s *Store) SetSeriesHidden(seriesID int64, hidden bool) error {
	_, err := s.db.Exec(`UPDATE books SET hidden = ? WHERE series_id = ?`, hidden, seriesID)
	return err
}

// SetSeriesRead marks every book in a series as read, or clears the
// series' progress when read is false.
func (s *Store) SetSeriesRead(userID, seriesID int64, read bool) error {
	if !read {
		_, err := s.db.Exec(`
			DELETE FROM progress WHERE user_id = ? AND book_id IN (SELECT id FROM books WHERE series_id = ?)`,
			userID, seriesID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO progress (user_id, book_id, page, completed, updated_at)
		SELECT ?1, b.id, b.page_count, 1, ?2 FROM books b WHERE b.series_id = ?3 AND `+visibleTo("b")+`
		ON CONFLICT (user_id, book_id) DO UPDATE SET
			page = excluded.page, completed = 1, updated_at = excluded.updated_at`,
		userID, time.Now().UTC().Format(timeMillis), seriesID)
	return err
}

type Stats struct {
	Libraries int   `json:"libraries"`
	Series    int   `json:"series"`
	Books     int   `json:"books"`
	Pages     int   `json:"pages"`
	Size      int64 `json:"size"`
	Users     int   `json:"users"`
	// Requests are the book requests waiting for an admin.
	Requests int `json:"requests"`
}

func (s *Store) Stats() (Stats, error) {
	var st Stats
	err := s.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM libraries), (SELECT COUNT(*) FROM series),
			COUNT(*), COALESCE(SUM(page_count), 0), COALESCE(SUM(size), 0),
			(SELECT COUNT(*) FROM users), (SELECT COUNT(*) FROM book_requests WHERE status = 'open')
		FROM books`).Scan(&st.Libraries, &st.Series, &st.Books, &st.Pages, &st.Size, &st.Users, &st.Requests)
	return st, err
}

type ScanResult struct {
	Series int `json:"series"`
	Books  int `json:"books"`
}

// Existing books keep their ID, so reading progress survives a rescan.
func (s *Store) SyncLibrary(libraryID int64, found []scanner.Series) (ScanResult, error) {
	return s.sync(libraryID, found, `
		DELETE FROM books WHERE id NOT IN (SELECT id FROM seen_books)
		AND series_id IN (SELECT id FROM series WHERE library_id = ?1);
		DELETE FROM series WHERE library_id = ?1 AND id NOT IN (SELECT id FROM seen_series)`, libraryID)
}

func (s *Store) SyncSeries(libraryID int64, found scanner.Series) error {
	_, err := s.sync(libraryID, []scanner.Series{found}, `
		DELETE FROM books WHERE id NOT IN (SELECT id FROM seen_books)
		AND series_id IN (SELECT id FROM series WHERE library_id = ?1 AND path = ?2);
		DELETE FROM series WHERE library_id = ?1 AND path = ?2
		AND NOT EXISTS (SELECT 1 FROM books WHERE series_id = series.id)`, libraryID, found.Path)
	return err
}

func (s *Store) sync(libraryID int64, found []scanner.Series, cleanup string, args ...any) (ScanResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ScanResult{}, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TEMP TABLE IF NOT EXISTS seen_series (id INTEGER PRIMARY KEY);
		CREATE TEMP TABLE IF NOT EXISTS seen_books (id INTEGER PRIMARY KEY);
		DELETE FROM seen_series;
		DELETE FROM seen_books`); err != nil {
		return ScanResult{}, err
	}

	var result ScanResult
	for _, sr := range found {
		if len(sr.Books) == 0 {
			continue
		}
		var seriesID int64
		err := tx.QueryRow(`
			INSERT INTO series (library_id, name, path) VALUES (?, ?, ?)
			ON CONFLICT (library_id, path) DO UPDATE SET name = excluded.name
			RETURNING id`, libraryID, sr.Name, sr.Path).Scan(&seriesID)
		if err != nil {
			return ScanResult{}, err
		}
		if _, err := tx.Exec(`INSERT INTO seen_series (id) VALUES (?)`, seriesID); err != nil {
			return ScanResult{}, err
		}
		result.Series++

		for i, b := range sr.Books {
			var bookID int64
			err := tx.QueryRow(`
				INSERT INTO books (series_id, title, path, format, size, mod_time, page_count, sort_order,
					author, language, released, description, isbn, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
				ON CONFLICT (path) DO UPDATE SET
					series_id = excluded.series_id, title = excluded.title, format = excluded.format,
					-- Scans don't count a PDF's pages (see PDFsWithoutPageCount),
					-- so an unchanged PDF keeps its count.
					page_count = CASE WHEN excluded.format = 'pdf' AND books.size = excluded.size
						AND books.mod_time = excluded.mod_time THEN books.page_count ELSE excluded.page_count END,
					size = excluded.size, mod_time = excluded.mod_time, sort_order = excluded.sort_order,
					author = excluded.author, language = excluded.language,
					released = excluded.released, description = excluded.description, isbn = excluded.isbn
				RETURNING id`,
				seriesID, b.Title, b.Path, b.Format, b.Size, b.ModTime.UTC().Format(time.RFC3339), b.PageCount, i,
				b.Metadata.Author, b.Metadata.Language, b.Metadata.Released, b.Metadata.Description, b.Metadata.ISBN,
			).Scan(&bookID)
			if err != nil {
				return ScanResult{}, err
			}
			if _, err := tx.Exec(`INSERT INTO seen_books (id) VALUES (?)`, bookID); err != nil {
				return ScanResult{}, err
			}
			result.Books++
		}
	}

	if _, err := tx.Exec(cleanup, args...); err != nil {
		return ScanResult{}, err
	}
	return result, tx.Commit()
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// BookLibrary returns the library a book is in.
func (s *Store) BookLibrary(bookID int64) (Library, error) {
	var l Library
	err := s.db.QueryRow(`SELECT l.id, l.name, l.path, l.allow_uploads FROM books b
		JOIN series s ON s.id = b.series_id JOIN libraries l ON l.id = s.library_id
		WHERE b.id = ?`, bookID).Scan(&l.ID, &l.Name, &l.Path, &l.AllowUploads)
	return l, notFound(err)
}

// DeleteBook removes a book with everything about it: progress, notes,
// covers and edits. A series left without books goes too.
func (s *Store) DeleteBook(bookID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var seriesID int64
	if err := tx.QueryRow(`DELETE FROM books WHERE id = ? RETURNING series_id`, bookID).Scan(&seriesID); err != nil {
		return notFound(err)
	}
	if _, err := tx.Exec(`DELETE FROM series WHERE id = ? AND NOT EXISTS (SELECT 1 FROM books WHERE series_id = ?)`,
		seriesID, seriesID); err != nil {
		return err
	}
	return tx.Commit()
}

// PDFsWithoutPageCount returns the IDs and paths of PDFs whose pages
// weren't counted yet; scans leave that to SetPageCount.
func (s *Store) PDFsWithoutPageCount() (map[int64]string, error) {
	rows, err := s.db.Query(`SELECT id, path FROM books WHERE format = 'pdf' AND page_count = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		out[id] = path
	}
	return out, rows.Err()
}

func (s *Store) SetPageCount(bookID int64, pages int) error {
	_, err := s.db.Exec(`UPDATE books SET page_count = ? WHERE id = ?`, pages, bookID)
	return err
}
