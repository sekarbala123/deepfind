package searcher

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob     string
		input    string
		expected bool
	}{
		{"*.txt", "hello.txt", true},
		{"*.txt", "hello.go", false},
		{"file?.txt", "file1.txt", true},
		{"file?.txt", "file12.txt", false},
		{"abc", "abc", true},
	}

	for _, tc := range tests {
		rxStr := globToRegex(tc.glob)
		if tc.expected {
			if !strings.Contains(rxStr, ".*") && tc.glob == "*.txt" {
				t.Errorf("globToRegex(%q) = %q, expected regex to contain wildcards", tc.glob, rxStr)
			}
		}
	}
}

func TestMatcherName(t *testing.T) {
	// Case sensitive, glob
	m, _ := NewMatcher(Options{NamePattern: "*.txt", IgnoreCase: false})
	if !m.MatchName("test.txt") {
		t.Error("expected test.txt to match *.txt")
	}
	if m.MatchName("test.TXT") {
		t.Error("expected test.TXT not to match *.txt (case sensitive)")
	}

	// Case insensitive, glob
	m2, _ := NewMatcher(Options{NamePattern: "*.txt", IgnoreCase: true})
	if !m2.MatchName("test.TXT") {
		t.Error("expected test.TXT to match *.txt (case insensitive)")
	}

	// Substring match
	m3, _ := NewMatcher(Options{NamePattern: "config", IgnoreCase: true})
	if !m3.MatchName("my_config_file.json") {
		t.Error("expected my_config_file.json to match config substring")
	}

	// Regex match
	m4, _ := NewMatcher(Options{NamePattern: `^file\d+\.go$`, UseRegex: true})
	if !m4.MatchName("file123.go") {
		t.Error("expected file123.go to match regex")
	}
	if m4.MatchName("myfile123.go") {
		t.Error("expected myfile123.go not to match anchored regex")
	}
}

func TestMatcherContent(t *testing.T) {
	// Substring match
	m, _ := NewMatcher(Options{ContentPattern: "hello", IgnoreCase: false})
	if !m.MatchContent("hello world") {
		t.Error("expected 'hello world' to match content 'hello'")
	}
	if m.MatchContent("HELLO world") {
		t.Error("expected 'HELLO world' not to match content 'hello' (case sensitive)")
	}

	// Regex match
	m2, _ := NewMatcher(Options{ContentPattern: `err = \w+`, UseRegex: true})
	if !m2.MatchContent("err = compute()") {
		t.Error("expected 'err = compute()' to match regex")
	}
}

func TestIsBinary(t *testing.T) {
	text := []byte("This is plain text with no null bytes.")
	if IsBinary(text) {
		t.Error("expected text to not be binary")
	}

	binary := []byte{0x00, 0x01, 0x02, 0x03}
	if !IsBinary(binary) {
		t.Error("expected binary payload to be binary")
	}

	invalidUTF8 := []byte{0xff, 0xfe, 0xfd}
	if !IsBinary(invalidUTF8) {
		t.Error("expected invalid UTF-8 bytes to be binary")
	}
}

// helper to create a ZIP file
func createTestZip(t *testing.T, path string, files map[string]string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = w.Write([]byte(content))
		if err != nil {
			t.Fatal(err)
		}
	}
}

// helper to create a TAR.GZ file
func createTestTarGz(t *testing.T, path string, files map[string]string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		err := tw.WriteHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tw.Write([]byte(content))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearchIntegration(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "deepfind-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write simple files
	err = os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world\nline two\nline three"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"port": 8080, "host": "localhost"}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create test archives
	zipFiles := map[string]string{
		"inner.txt":  "this is inside a zip file\nkeyword found here",
		"config.xml": "<config><port>9090</port></config>",
	}
	createTestZip(t, filepath.Join(tmpDir, "archive.zip"), zipFiles)

	tarFiles := map[string]string{
		"readme.md": "tar gz readme content\nkeyword also here",
		"sub/a.go":  "package sub",
	}
	createTestTarGz(t, filepath.Join(tmpDir, "archive.tar.gz"), tarFiles)

	// Test case 1: simple filename glob
	t.Run("NameGlob", func(t *testing.T) {
		opts := Options{
			StartPath:   tmpDir,
			NamePattern: "*.txt",
		}
		results := make(chan Result)
		go Search(context.Background(), opts, results)

		var matched []string
		for r := range results {
			if r.Error != "" {
				t.Errorf("Unexpected error: %s", r.Error)
			}
			matched = append(matched, filepath.Base(r.Path))
		}

		if len(matched) != 1 || matched[0] != "hello.txt" {
			t.Errorf("Expected only hello.txt, got %v", matched)
		}
	})

	// Test case 2: text content search in normal files
	t.Run("ContentSearchNormal", func(t *testing.T) {
		opts := Options{
			StartPath:      tmpDir,
			ContentPattern: "line",
		}
		results := make(chan Result)
		go Search(context.Background(), opts, results)

		var matches []Result
		for r := range results {
			if r.Error != "" {
				t.Errorf("Unexpected error: %s", r.Error)
			}
			matches = append(matches, r)
		}

		// should find 2 matches in hello.txt (line two, line three)
		if len(matches) != 2 {
			t.Errorf("Expected 2 content matches, got %d", len(matches))
		}
		for _, m := range matches {
			if filepath.Base(m.Path) != "hello.txt" {
				t.Errorf("Expected match in hello.txt, got %s", m.Path)
			}
			if !strings.Contains(m.LineContent, "line") {
				t.Errorf("Expected matched line to contain 'line', got %q", m.LineContent)
			}
		}
	})

	// Test case 3: archive search filename & content
	t.Run("ArchiveSearch", func(t *testing.T) {
		opts := Options{
			StartPath:      tmpDir,
			SearchArchives: true,
			ContentPattern: "keyword",
		}
		results := make(chan Result)
		go Search(context.Background(), opts, results)

		var matches []Result
		for r := range results {
			if r.Error != "" {
				t.Errorf("Unexpected error: %s", r.Error)
			}
			matches = append(matches, r)
		}

		// Should find:
		// 1. inner.txt inside archive.zip (has "keyword")
		// 2. readme.md inside archive.tar.gz (has "keyword")
		if len(matches) != 2 {
			t.Errorf("Expected 2 archive matches, got %d", len(matches))
		}

		zipFound, tarFound := false, false
		for _, m := range matches {
			if !m.IsArchive {
				t.Errorf("Expected match to be marked as archive")
			}
			if filepath.Base(m.ArchivePath) == "archive.zip" && m.InnerPath == "inner.txt" {
				zipFound = true
			}
			if filepath.Base(m.ArchivePath) == "archive.tar.gz" && m.InnerPath == "readme.md" {
				tarFound = true
			}
		}

		if !zipFound || !tarFound {
			t.Errorf("Did not find matches in both archives. ZipFound: %v, TarFound: %v", zipFound, tarFound)
		}
	})
}

func TestGetMatchingArchiveFormat(t *testing.T) {
	formats := []ArchiveFormat{
		{Extension: ".zip", Type: "zip", Enabled: true},
		{Extension: ".tar.gz", Type: "tar", Enabled: true},
		{Extension: ".gz", Type: "tar", Enabled: false},
		{Extension: ".apk", Type: "zip", Enabled: true},
	}

	tests := []struct {
		filename string
		expected string
		found    bool
	}{
		{"test.zip", ".zip", true},
		{"test.apk", ".apk", true},
		{"test.tar.gz", ".tar.gz", true},
		{"test.gz", "", false}, // disabled
		{"test.txt", "", false},
	}

	for _, tc := range tests {
		fmt, ok := getMatchingArchiveFormat(tc.filename, formats)
		if ok != tc.found {
			t.Errorf("getMatchingArchiveFormat(%q) found = %v, expected %v", tc.filename, ok, tc.found)
		}
		if ok && fmt.Extension != tc.expected {
			t.Errorf("getMatchingArchiveFormat(%q) ext = %q, expected %q", tc.filename, fmt.Extension, tc.expected)
		}
	}
}



// TestExportResultsToCSV tests CSV export functionality
func TestExportResultsToCSV(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	results := []Result{
		{Path: "/home/user/file.txt", IsArchive: false, IsContentMatch: true, LineNumber: 5, LineContent: "test content"},
		{Path: "/home/user/archive.zip", IsArchive: true, ArchivePath: "/home/user/archive.zip", InnerPath: "inner.txt", IsContentMatch: true, LineNumber: 10, LineContent: "archive content"},
		{Path: "/home/user/bad.gz", IsArchive: true, ArchivePath: "/home/user/bad.gz", Error: "(skipped)"},
	}

	if err := ExportResultsToCSV(results, tmpPath); err != nil {
		t.Fatalf("ExportResultsToCSV failed: %v", err)
	}

	// Verify file was created and contains data
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("CSV file is empty")
	}

	// Verify CSV contains header
	csv := string(data)
	if !strings.Contains(csv, "path,is_archive,archive_path,inner_path,is_content_match,line_number,line_content,error") {
		t.Errorf("CSV header not found. Got: %s", csv)
	}

	// Verify CSV contains data rows
	if !strings.Contains(csv, "/home/user/file.txt") {
		t.Errorf("First result not found in CSV")
	}

	if !strings.Contains(csv, "/home/user/archive.zip") {
		t.Errorf("Archive result not found in CSV")
	}
}
