package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deepfind/searcher"
	"deepfind/web"
)

// Terminal color codes
var (
	colorReset  = ""
	colorBold   = ""
	colorRed    = ""
	colorGreen  = ""
	colorYellow = ""
	colorBlue   = ""
	colorPurple = ""
	colorCyan   = ""
)

func initColors() {
	// Disable colors if NO_COLOR is set or TERM is dumb
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return
	}
	// Check if output is a terminal (standard unix check)
	fileInfo, err := os.Stdout.Stat()
	if err == nil && (fileInfo.Mode()&os.ModeCharDevice) != 0 {
		colorReset = "\033[0m"
		colorBold = "\033[1m"
		colorRed = "\033[31m"
		colorGreen = "\033[32m"
		colorYellow = "\033[33m"
		colorBlue = "\033[34m"
		colorPurple = "\033[35m"
		colorCyan = "\033[36m"
	}
}

func main() {
	// Initialize terminal colors
	initColors()

	// Define command-line flags
	startPath := flag.String("path", ".", "Directory path to start searching from")
	namePattern := flag.String("name", "", "Filename pattern (glob wildcards like *.json or regex)")
	contentPattern := flag.String("content", "", "Text content to search for inside files")
	searchArchives := flag.Bool("archive", false, "Search inside ZIP, JAR, WAR, and TAR archives")
	useRegex := flag.Bool("regex", false, "Interpret filename and content patterns as regular expressions")
	ignoreCase := flag.Bool("ignore-case", true, "Perform case-insensitive matching")
	runUI := flag.Bool("ui", false, "Launch the local Web UI dashboard")
	exportCSV := flag.String("export-csv", "", "Export search results to CSV file")
	port := flag.Int("port", 8080, "Web UI port")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%sdeepfind%s - Multiplatform File & Archive Search Utility\n\n", colorBold, colorReset)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  CLI Search:  deepfind -name <pattern> -content <text> [options]\n")
		fmt.Fprintf(os.Stderr, "  Web UI Mode: deepfind -ui [-port <number>]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// If UI is requested or no flags were passed at all, start Web UI
	if *runUI {
		fmt.Printf("%sStarting DeepFind Web UI...%s\n", colorGreen, colorReset)
		err := web.StartServer(*port, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sServer error: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		return
	}

	// Validate CLI search requirements (must have at least name or content pattern if not running UI)
	if *namePattern == "" && *contentPattern == "" {
		// If no parameters are passed, print usage and guide user to run with -ui
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\n%sTip: Run 'deepfind -ui' to open the interactive browser-based dashboard!%s\n", colorCyan, colorReset)
		os.Exit(1)
	}

	// Resolve absolute path for start path
	absPath, err := filepath.Abs(*startPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sInvalid start path: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	opts := searcher.Options{
		StartPath:      absPath,
		NamePattern:    *namePattern,
		ContentPattern: *contentPattern,
		SearchArchives: *searchArchives,
		UseRegex:       *useRegex,
		IgnoreCase:     *ignoreCase,
	}

	// Display searching message
	fmt.Printf("%sScanning directory:%s %s\n", colorBold, colorReset, absPath)
	if opts.NamePattern != "" {
		fmt.Printf("  %sFilename pattern:%s %s (regex: %v, ignoreCase: %v)\n", colorBold, colorReset, opts.NamePattern, opts.UseRegex, opts.IgnoreCase)
	}
	if opts.ContentPattern != "" {
		fmt.Printf("  %sContent pattern:%s %s (regex: %v, ignoreCase: %v)\n", colorBold, colorReset, opts.ContentPattern, opts.UseRegex, opts.IgnoreCase)
	}
	if opts.SearchArchives {
		fmt.Printf("  %sArchive search:%s enabled (.zip, .jar, .tar, .tgz)\n", colorBold, colorReset)
	}
	fmt.Println()

	// Launch CLI search
	startTime := time.Now()
	resultsChan := make(chan searcher.Result, 100)
	ctx := context.Background()

	go searcher.Search(ctx, opts, resultsChan)

	matchCount := 0
	var allResults []searcher.Result
	for res := range resultsChan {
		allResults = append(allResults, res)
		if res.Error != "" {
			fmt.Fprintf(os.Stderr, "%s[ERROR] %s: %s%s\n", colorRed, res.Path, res.Error, colorReset)
			continue
		}

		matchCount++
		// Format and print each result
		if res.IsArchive {
			// e.g. [Archive] /path/to/file.zip -> inner/path.txt
			fmt.Printf("%s[Archive]%s %s%s%s -> %s%s%s",
				colorPurple, colorReset,
				colorYellow, filepath.Base(res.ArchivePath), colorReset,
				colorBlue, res.InnerPath, colorReset,
			)
		} else {
			// regular file
			fmt.Printf("%s%s%s", colorBlue, res.Path, colorReset)
		}

		// Print content snippet if matched inside file
		if res.IsContentMatch {
			fmt.Printf(":%s%d%s: %s", colorYellow, res.LineNumber, colorReset, highlightTerminal(res.LineContent, *contentPattern))
		}
		fmt.Println()
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n%sSearch completed in %s. Found %d matches.%s\n", colorGreen, elapsed.Round(time.Millisecond), matchCount, colorReset)

	// Export to CSV if requested
	if *exportCSV != "" {
		if err := searcher.ExportResultsToCSV(allResults, *exportCSV); err != nil {
			fmt.Fprintf(os.Stderr, "%sError exporting CSV: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%sResults exported to: %s%s\n", colorGreen, *exportCSV, colorReset)
	}
}

func highlightTerminal(line string, pattern string) string {
	if pattern == "" || colorCyan == "" {
		return line
	}
	// Case-insensitive replacement for simple terminal highlight
	lowerLine := strings.ToLower(line)
	lowerPattern := strings.ToLower(pattern)
	
	// If it doesn't contain, just return
	idx := strings.Index(lowerLine, lowerPattern)
	if idx == -1 {
		return line
	}

	var buf strings.Builder
	lastIdx := 0
	for idx != -1 {
		buf.WriteString(line[lastIdx : lastIdx+idx])
		matchedText := line[lastIdx+idx : lastIdx+idx+len(pattern)]
		buf.WriteString(colorCyan + colorBold + matchedText + colorReset)
		
		lastIdx = lastIdx + idx + len(pattern)
		lowerLine = lowerLine[idx+len(pattern):]
		idx = strings.Index(lowerLine, lowerPattern)
	}
	buf.WriteString(line[lastIdx:])
	return buf.String()
}
