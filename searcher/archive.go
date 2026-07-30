package searcher

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// isValidArchivePath validates that an archive entry path is safe and doesn't contain traversal attempts.
// Returns true if the path is valid, false if it contains dangerous patterns.
func isValidArchivePath(entryPath string) bool {
	// Reject paths with explicit parent directory references
	if strings.Contains(entryPath, "..") {
		return false
	}
	// Ensure the path is not absolute
	if filepath.IsAbs(entryPath) {
		return false
	}
	// Clean the path and verify it doesn't escape
	cleanPath := filepath.Clean(entryPath)
	if strings.HasPrefix(cleanPath, "..") {
		return false
	}
	return true
}

// SearchArchiveFile opens the archive and searches its contents.
func (m *Matcher) SearchArchiveFile(archivePath string, archiveType string, results chan<- Result) {
	if archiveType == "zip" {
		m.searchZip(archivePath, results)
	} else if archiveType == "tar" {
		m.searchTar(archivePath, results)
	}
}

func isGzip(f io.ReadSeeker) bool {
	var magic [2]byte
	n, err := f.Read(magic[:])
	_, _ = f.Seek(0, io.SeekStart)
	if err != nil || n < 2 {
		return false
	}
	return magic[0] == 0x1f && magic[1] == 0x8b
}

func (m *Matcher) searchZip(archivePath string, results chan<- Result) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		results <- Result{Path: archivePath, Error: "Zip open error: " + err.Error()}
		return
	}
	defer r.Close()

	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Validate path to prevent traversal attacks
		if !isValidArchivePath(f.Name) {
			continue
		}

		baseName := filepath.Base(f.Name)
		if m.MatchName(baseName) {
			if m.opts.ContentPattern != "" {
				rc, err := f.Open()
				if err != nil {
					results <- Result{Path: archivePath, IsArchive: true, ArchivePath: archivePath, InnerPath: f.Name, Error: "Zip entry open error: " + err.Error()}
					continue
				}
				m.ScanFileContent(rc, archivePath, archivePath, f.Name, results)
				rc.Close()
			} else {
				// Just name matched inside archive
				results <- Result{
					Path:        archivePath,
					IsArchive:   true,
					ArchivePath: archivePath,
					InnerPath:   f.Name,
				}
			}
		}
	}
}

func (m *Matcher) searchTar(archivePath string, results chan<- Result) {
	f, err := os.Open(archivePath)
	if err != nil {
		results <- Result{Path: archivePath, Error: "Tar open error: " + err.Error()}
		return
	}
	defer f.Close()

	var reader io.Reader = f

	// If gzipped, wrap with gzip reader
	if isGzip(f) {
		gr, err := gzip.NewReader(f)
		if err != nil {
			results <- Result{Path: archivePath, Error: "Gzip reader error: " + err.Error()}
			return
		}
		defer gr.Close()
		reader = gr
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Error reading TAR: report and stop
			// This conservative approach ensures data integrity by not returning partial/incomplete result sets.
			// Future enhancement: add optional --skip-corrupted flag to continue on read errors.
			results <- Result{Path: archivePath, Error: "Tar read error: " + err.Error()}
			break
		}

		// Skip directories and special file types (symlinks, hard links, sparse files, etc.)
		if hdr.Typeflag == tar.TypeDir ||
			hdr.Typeflag == tar.TypeSymlink ||
			hdr.Typeflag == tar.TypeLink ||
			hdr.Typeflag == tar.TypeGNUSparse ||
			hdr.Typeflag == tar.TypeGNULongName ||
			hdr.Typeflag == tar.TypeGNULongLink {
			continue
		}

		// Validate path to prevent traversal attacks
		if !isValidArchivePath(hdr.Name) {
			continue
		}

		baseName := filepath.Base(hdr.Name)
		if m.MatchName(baseName) {
			if m.opts.ContentPattern != "" {
				// Scan file content directly from tar reader stream
				m.ScanFileContent(tr, archivePath, archivePath, hdr.Name, results)
			} else {
				// Just name matched inside archive
				results <- Result{
					Path:        archivePath,
					IsArchive:   true,
					ArchivePath: archivePath,
					InnerPath:   hdr.Name,
				}
			}
		}
	}
}
