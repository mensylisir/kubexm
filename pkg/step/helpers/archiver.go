package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"
)

type ProgressFunc func(name string, totalBytes int64)

type Archiver struct {
	OverwriteExisting bool
	Progress          ProgressFunc
}

type Option func(*Archiver)

func NewArchiver(opts ...Option) *Archiver {
	ar := &Archiver{
		OverwriteExisting: false,
	}
	for _, opt := range opts {
		opt(ar)
	}
	return ar
}

func WithOverwrite(overwrite bool) Option {
	return func(a *Archiver) {
		a.OverwriteExisting = overwrite
	}
}

func WithProgress(p ProgressFunc) Option {
	return func(a *Archiver) {
		a.Progress = p
	}
}

func (a *Archiver) Extract(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file does not exist: %s", source)
		}
		return fmt.Errorf("failed to stat source file %s: %w", source, err)
	}
	if info.IsDir() {
		return fmt.Errorf("source is a directory, not a file: %s", source)
	}
	archiveTotalSize := info.Size()

	walkFn := func(f archiver.File) error {

		defer f.Close()

		if a.Progress != nil {
			a.Progress(f.Name(), archiveTotalSize)
		}

		destPath := filepath.Join(destination, f.Name())

		if !a.OverwriteExisting {
			if _, err := os.Stat(destPath); !os.IsNotExist(err) {
				return nil
			}
		}

		if f.IsDir() {
			return os.MkdirAll(destPath, f.Mode())
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, f)
		if err != nil {
			return fmt.Errorf("failed to write to destination file %s: %w", destPath, err)
		}

		return nil
	}

	err = archiver.Walk(source, walkFn)
	if err != nil {
		return fmt.Errorf("failed to walk archive %s: %w", source, err)
	}

	return nil
}

func (a *Archiver) Compress(sources []string, destination string) error {
	for _, src := range sources {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return fmt.Errorf("source to compress does not exist: %s", src)
		}
	}
	if a.OverwriteExisting {
		if _, err := os.Stat(destination); err == nil {
			if err := os.Remove(destination); err != nil {
				return fmt.Errorf("failed to remove existing destination archive %s for overwrite: %w", destination, err)
			}
		}
	}
	return archiver.Archive(sources, destination)
}
