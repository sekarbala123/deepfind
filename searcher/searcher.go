package searcher

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ArchiveFormat represents an archive file extension and its decompression type.
type ArchiveFormat struct {
	Extension string `json:"extension"`
	Type      string `json:"type"` // "zip" or "tar"
	Enabled   bool   `json:"enabled"`
}

// Options defines the parameters for a file search.
type Options struct {
	StartPath      string          `json:"startPath"`
	NamePattern    string          `json:"namePattern"`
	ContentPattern string          `json:"contentPattern"`
	SearchArchives bool            `json:"searchArchives"`
	ArchiveFormats []ArchiveFormat `json:"archiveFormats"`
	UseRegex       bool            `json:"useRegex"`
	IgnoreCase     bool            `json:"ignoreCase"`
}

var DefaultArchiveFormats = []ArchiveFormat{
	{Extension: ".zip", Type: "zip", Enabled: true},
	{Extension: ".jar", Type: "zip", Enabled: true},
	{Extension: ".war", Type: "zip", Enabled: true},
	{Extension: ".tar", Type: "tar", Enabled: true},
	{Extension: ".tgz", Type: "tar", Enabled: true},
	{Extension: ".tar.gz", Type: "tar", Enabled: true},
	{Extension: ".gz", Type: "tar", Enabled: true},
}

// getMatchingArchiveFormat returns the matched archive format for a given filename, if any.
func getMatchingArchiveFormat(filename string, formats []ArchiveFormat) (ArchiveFormat, bool) {
	filenameLower := strings.ToLower(filename)
	var bestMatch ArchiveFormat
	var found bool
	for _, fmt := range formats {
		if !fmt.Enabled {
			continue
		}
		extLower := strings.ToLower(fmt.Extension)
		if strings.HasSuffix(filenameLower, extLower) {
			if !found || len(extLower) > len(bestMatch.Extension) {
				bestMatch = fmt
				found = true
			}
		}
	}
	return bestMatch, found
}

// Result represents a search match.
type Result struct {
	Path           string `json:"path"`
	IsArchive      bool   `json:"isArchive"`
	ArchivePath    string `json:"archivePath,omitempty"`
	InnerPath      string `json:"innerPath,omitempty"`
	IsContentMatch bool   `json:"isContentMatch"`
	LineNumber     int    `json:"lineNumber,omitempty"`
	LineContent    string `json:"lineContent,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Matcher wraps compiled regexes for name and content matching.
type Matcher struct {
	nameRx    *regexp.Regexp
	contentRx *regexp.Regexp
	opts      Options
}

// NewMatcher compiles the patterns in Options.
func NewMatcher(opts Options) (*Matcher, error) {
	m := &Matcher{opts: opts}

	// 1. Compile name pattern
	if opts.NamePattern != "" {
		if opts.UseRegex {
			pattern := opts.NamePattern
			if opts.IgnoreCase {
				pattern = "(?i)" + pattern
			}
			rx, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			m.nameRx = rx
		} else {
			// If it contains glob characters, convert glob to regex
			if strings.ContainsAny(opts.NamePattern, "*?") {
				pattern := globToRegex(opts.NamePattern)
				if opts.IgnoreCase {
					pattern = "(?i)" + pattern
				}
				rx, err := regexp.Compile(pattern)
				if err != nil {
					return nil, err
				}
				m.nameRx = rx
			}
		}
	}

	// 2. Compile content pattern
	if opts.ContentPattern != "" {
		if opts.UseRegex {
			pattern := opts.ContentPattern
			if opts.IgnoreCase {
				pattern = "(?i)" + pattern
			}
			rx, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			m.contentRx = rx
		}
	}

	return m, nil
}

// MatchName checks if a filename matches the NamePattern.
func (m *Matcher) MatchName(name string) bool {
	if m.opts.NamePattern == "" {
		return true
	}

	// If regex is compiled, use it
	if m.nameRx != nil {
		return m.nameRx.MatchString(name)
	}

	// Substring match
	if m.opts.IgnoreCase {
		return strings.Contains(strings.ToLower(name), strings.ToLower(m.opts.NamePattern))
	}
	return strings.Contains(name, m.opts.NamePattern)
}

// MatchContent checks if a line matches the ContentPattern.
func (m *Matcher) MatchContent(line string) bool {
	if m.opts.ContentPattern == "" {
		return true
	}

	if m.contentRx != nil {
		return m.contentRx.MatchString(line)
	}

	// Substring match
	if m.opts.IgnoreCase {
		return strings.Contains(strings.ToLower(line), strings.ToLower(m.opts.ContentPattern))
	}
	return strings.Contains(line, m.opts.ContentPattern)
}

// globToRegex converts a wildcard glob pattern (*, ?) to a regex string.
func globToRegex(glob string) string {
	var buf bytes.Buffer
	buf.WriteString("^")
	for _, r := range glob {
		switch r {
		case '*':
			buf.WriteString(".*")
		case '?':
			buf.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			buf.WriteString("\\")
			buf.WriteRune(r)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteString("$")
	return buf.String()
}

// IsBinary detects if the given header bytes represent binary content.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Check first 512 bytes for NULL bytes or non-UTF8 characters
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true // Null byte means binary
		}
	}
	// Also check if it's valid UTF-8
	return !utf8.Valid(data[:limit])
}

// ScanFileContent searches inside a file's content and streams matches.
func (m *Matcher) ScanFileContent(r io.Reader, filePath string, archivePath string, innerPath string, results chan<- Result) {
	// Read a header first to check if it's binary
	header := make([]byte, 1024)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		results <- Result{Path: filePath, ArchivePath: archivePath, InnerPath: innerPath, Error: err.Error()}
		return
	}
	header = header[:n]

	if IsBinary(header) {
		// Skip binary files for content search
		return
	}

	// Combine header and remainder of reader
	var fullReader io.Reader
	if len(header) > 0 {
		fullReader = io.MultiReader(bytes.NewReader(header), r)
	} else {
		fullReader = r
	}

	scanner := bufio.NewScanner(fullReader)
	// Set a reasonable buffer size limit to prevent crash on huge lines
	const maxCapacity = 64 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if m.MatchContent(line) {
			res := Result{
				Path:           filePath,
				IsContentMatch: true,
				LineNumber:     lineNum,
				LineContent:    strings.TrimSpace(line),
			}
			if archivePath != "" {
				res.IsArchive = true
				res.ArchivePath = archivePath
				res.InnerPath = innerPath
			}
			results <- res
		}
	}

	if err := scanner.Err(); err != nil && err != bufio.ErrTooLong {
		results <- Result{Path: filePath, ArchivePath: archivePath, InnerPath: innerPath, Error: err.Error()}
	}
}

// Search walks the directory tree and sends search results to the channel.
func Search(ctx context.Context, opts Options, results chan<- Result) {
	defer close(results)

	m, err := NewMatcher(opts)
	if err != nil {
		results <- Result{Error: "Invalid patterns: " + err.Error()}
		return
	}

	startPath := opts.StartPath
	if startPath == "" {
		startPath = "."
	}

	formats := opts.ArchiveFormats
	if len(formats) == 0 {
		formats = DefaultArchiveFormats
	}

	err = filepath.WalkDir(startPath, func(path string, d os.DirEntry, err error) error {
		// Handle context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// Report error walking this path but continue
			results <- Result{Path: path, Error: err.Error()}
			return nil
		}

		if d.IsDir() {
			// Skip hidden directories like .git
			name := d.Name()
			if name != "." && name != ".." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		filename := d.Name()

		if opts.SearchArchives {
			if fmt, ok := getMatchingArchiveFormat(filename, formats); ok {
				// Delegate archive searching
				m.SearchArchiveFile(path, fmt.Type, results)
				return nil
			}
		}

		// Regular file search
		if m.MatchName(filename) {
			if opts.ContentPattern != "" {
				// We need to scan content
				file, err := os.Open(path)
				if err != nil {
					results <- Result{Path: path, Error: err.Error()}
					return nil
				}
				defer file.Close()
				m.ScanFileContent(file, path, "", "", results)
			} else {
				// Just name matched, report it!
				results <- Result{
					Path: path,
				}
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		results <- Result{Error: "Walk error: " + err.Error()}
	}
}


