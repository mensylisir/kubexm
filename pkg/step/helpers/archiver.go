package helpers

import (
	"archive/tar"
	"compress/gzip"
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

func CompressTarGz(sourceDir, destPath string) error {
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	gzipWriter := gzip.NewWriter(destFile)
	if gzipWriter == nil {
		return fmt.Errorf("failed to create gzip writer")
	}
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(sourceDir, func(currentPath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", currentPath, err)
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", currentPath, err)
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", fileInfo.Name(), err)
		}
		header.Name = relPath

		if fileInfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(currentPath)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", currentPath, err)
			}
			header.Linkname = linkTarget
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", header.Name, err)
		}

		if !fileInfo.IsDir() && fileInfo.Mode().IsRegular() {
			file, err := os.Open(currentPath)
			if err != nil {
				return fmt.Errorf("failed to open file %s for archiving: %w", currentPath, err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file content for %s: %w", currentPath, err)
			}
		}

		return nil
	})
}

func ExtractTarGz(sourcePath, destDir string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source tar.gz file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	gzipReader, err := gzip.NewReader(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for %s: %w", sourcePath, err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read next tar header: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write content to file %s: %w", targetPath, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}
		default:
			fmt.Printf("unsupported file type for %s: %c\n", header.Name, header.Typeflag)
		}
	}
	return nil
}
