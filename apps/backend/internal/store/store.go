// Package store reads and writes the database.
package store

import (
	"cmp"
	"database/sql"
	"errors"
	"time"

	"novil/backend/internal/scanner"
)

var ErrNotFound = errors.New("not found")

type Library struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	AllowUploads bool   `json:"allowUploads"`
}

type Series struct {
	ID          int64  `json:"id"`
	LibraryID   int64  `json:"libraryId"`
	Name        string `json:"name"`
	BookCount   int    `json:"bookCount"`
	CoverBookID *int64 `json:"coverBookId"`
}

type Book struct {
	ID        int64  `json:"id"`
	SeriesID  int64  `json:"seriesId"`
	Title     string `json:"title"`
	Format    string `json:"format"`
	Size      int64  `json:"size"`
	PageCount int    `json:"pageCount"`
	ReadPage  int    `json:"readPage"`
	Completed bool   `json:"completed"`
	Path      string `json:"-"`
}

type Progress struct {
	BookID    int64     `json:"bookId"`
	Page      int       `json:"page"`
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

const seriesSelect = `
	SELECT s.id, s.library_id, s.name, COUNT(b.id),
		(SELECT id FROM books WHERE series_id = s.id AND format = 'cbz' ORDER BY sort_order LIMIT 1)
	FROM series s LEFT JOIN books b ON b.series_id = s.id`

func (s *Store) ListSeries(libraryID int64) ([]Series, error) {
	rows, err := s.db.Query(seriesSelect+`
		WHERE ? = 0 OR s.library_id = ?
		GROUP BY s.id ORDER BY s.name COLLATE NOCASE`, libraryID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Series{}
	for rows.Next() {
		var sr Series
		if err := rows.Scan(&sr.ID, &sr.LibraryID, &sr.Name, &sr.BookCount, &sr.CoverBookID); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

func (s *Store) GetSeries(id int64) (Series, error) {
	var sr Series
	err := s.db.QueryRow(seriesSelect+` WHERE s.id = ? GROUP BY s.id`, id).
		Scan(&sr.ID, &sr.LibraryID, &sr.Name, &sr.BookCount, &sr.CoverBookID)
	return sr, notFound(err)
}

const bookSelect = `
	SELECT b.id, b.series_id, b.title, b.format, b.size, b.page_count,
		COALESCE(p.page, 0), COALESCE(p.completed, 0), b.path
	FROM books b LEFT JOIN progress p ON p.book_id = b.id`

func scanBook(row interface{ Scan(...any) error }) (Book, error) {
	var b Book
	err := row.Scan(&b.ID, &b.SeriesID, &b.Title, &b.Format, &b.Size, &b.PageCount, &b.ReadPage, &b.Completed, &b.Path)
	return b, err
}

func (s *Store) ListBooks(seriesID int64) ([]Book, error) {
	rows, err := s.db.Query(bookSelect+` WHERE b.series_id = ? ORDER BY b.sort_order`, seriesID)
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

func (s *Store) GetBook(id int64) (Book, error) {
	b, err := scanBook(s.db.QueryRow(bookSelect+` WHERE b.id = ?`, id))
	return b, notFound(err)
}

func (s *Store) GetBookByPath(path string) (Book, error) {
	b, err := scanBook(s.db.QueryRow(bookSelect+` WHERE b.path = ?`, path))
	return b, notFound(err)
}

func (s *Store) GetProgress(bookID int64) (Progress, error) {
	p := Progress{BookID: bookID}
	var updated string
	err := s.db.QueryRow(`SELECT page, completed, updated_at FROM progress WHERE book_id = ?`, bookID).
		Scan(&p.Page, &p.Completed, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updated)
	return p, err
}

func (s *Store) SetProgress(p Progress) (Progress, error) {
	p.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	_, err := s.db.Exec(`
		INSERT INTO progress (book_id, page, completed, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (book_id) DO UPDATE SET
			page = excluded.page, completed = excluded.completed, updated_at = excluded.updated_at`,
		p.BookID, p.Page, p.Completed, p.UpdatedAt.Format(time.RFC3339))
	return p, err
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
				INSERT INTO books (series_id, title, path, format, size, mod_time, page_count, sort_order)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (path) DO UPDATE SET
					series_id = excluded.series_id, title = excluded.title, format = excluded.format,
					size = excluded.size, mod_time = excluded.mod_time,
					page_count = excluded.page_count, sort_order = excluded.sort_order
				RETURNING id`,
				seriesID, b.Title, b.Path, b.Format, b.Size, b.ModTime.UTC().Format(time.RFC3339), b.PageCount, i,
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
