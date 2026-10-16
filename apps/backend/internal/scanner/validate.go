package scanner

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"novil/backend/internal/archive"
)

func Validate(path, format string) error {
	switch format {
	case "cbz":
		c, err := archive.OpenCBZ(path)
		if err != nil {
			return err
		}
		defer c.Close()
		if c.PageCount() == 0 {
			return errors.New("no images in archive")
		}
		return nil
	case "epub":
		return validateEPUB(path)
	case "pdf":
		return validatePDF(path)
	}
	return fmt.Errorf("unsupported format %q", format)
}

func validateEPUB(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "mimetype" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, 64))
		if err != nil {
			return err
		}
		if string(bytes.TrimSpace(data)) != "application/epub+zip" {
			return errors.New("wrong mimetype")
		}
		return nil
	}
	return errors.New("no mimetype entry")
}

func validatePDF(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 5)
	if _, err := io.ReadFull(f, header); err != nil || string(header) != "%PDF-" {
		return errors.New("missing PDF header")
	}
	return nil
}
