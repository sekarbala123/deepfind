package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"deepfind/searcher"
)

//go:embed assets/*
var assetsFS embed.FS

// StartServer launches the local web server.
func StartServer(port int, autoOpen bool) error {
	// Setup handlers
	subFS, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return fmt.Errorf("failed to load embedded assets: %w", err)
	}

	// Serve CSS/JS assets
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(subFS))))

	// Serve main HTML page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		indexBytes, err := assetsFS.ReadFile("assets/index.html")
		if err != nil {
			http.Error(w, "Assets not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexBytes)
	})

	// Platform details endpoint
	http.HandleFunc("/api/platform", handlePlatform)

	// SSE Search endpoint
	http.HandleFunc("/api/search", handleSearch)

	// Directory browser endpoint
	http.HandleFunc("/api/browse", handleBrowse)

	// Bind to port (find first available if 0 or default is taken)
	listener, actualPort, err := listenOnPort(port)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", actualPort)
	fmt.Printf("Web UI server running at: %s\n", url)

	if autoOpen {
		go func() {
			time.Sleep(300 * time.Millisecond) // brief sleep to let server bind fully
			if err := OpenBrowser(url); err != nil {
				fmt.Printf("Could not open browser automatically: %v\n", err)
			}
		}()
	}

	return http.Serve(listener, nil)
}

func listenOnPort(port int) (net.Listener, int, error) {
	if port > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return ln, port, nil
		}
		fmt.Printf("Port %d is occupied, seeking next available port...\n", port)
	}

	// Try starting from 8080 and seek
	for p := 8080; p < 9000; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			return ln, p, nil
		}
	}

	return nil, 0, fmt.Errorf("could not find any available TCP port to bind")
}

func handlePlatform(w http.ResponseWriter, r *http.Request) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"currentDir": cwd,
	})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	// Set headers for Server-Sent Events (SSE)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	q := r.URL.Query()
	var archiveFormats []searcher.ArchiveFormat
	if formatsJSON := q.Get("archiveFormats"); formatsJSON != "" {
		_ = json.Unmarshal([]byte(formatsJSON), &archiveFormats)
	}

	opts := searcher.Options{
		StartPath:      q.Get("startPath"),
		NamePattern:    q.Get("namePattern"),
		ContentPattern: q.Get("contentPattern"),
		SearchArchives: parseBool(q.Get("searchArchives")),
		ArchiveFormats: archiveFormats,
		UseRegex:       parseBool(q.Get("useRegex")),
		IgnoreCase:     parseBool(q.Get("ignoreCase")),
	}

	// Channel for results stream
	resultsChan := make(chan searcher.Result, 100)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Launch concurrent search
	go searcher.Search(ctx, opts, resultsChan)

	// Stream results to client in real-time
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected, stop searching
			return
		case res, ok := <-resultsChan:
			if !ok {
				// Search finished, write final [DONE] event
				fmt.Fprint(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			// Format result as JSON
			data, err := json.Marshal(res)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func parseBool(val string) bool {
	b, _ := strconv.ParseBool(val)
	return b
}

// OpenBrowser opens the browser window for the specified URL on different platforms.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		// Check if it's running inside WSL
		if isWSL() {
			cmd = exec.Command("cmd.exe", "/c", "start", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// Check /proc/version for Microsoft
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	path, err := ChooseFolder()
	
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      "",
			"cancelled": true,
			"error":     err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path": path,
	})
}

func ChooseFolder() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return chooseFolderMac()
	case "windows":
		return chooseFolderWindows()
	case "linux":
		return chooseFolderLinux()
	default:
		return "", fmt.Errorf("unsupported platform for native folder dialog: %s", runtime.GOOS)
	}
}

func chooseFolderMac() (string, error) {
	script := `POSIX path of (choose folder with prompt "Select directory to search:")`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func chooseFolderWindows() (string, error) {
	psCmd := `
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.FolderBrowserDialog
$f.ShowNewFolderButton = $true
$f.Description = "Select search directory"
$res = $f.ShowDialog()
if ($res -eq 1) {
    Write-Output $f.SelectedPath
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func chooseFolderLinux() (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command("zenity", "--file-selection", "--directory", "--title=Select search directory")
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command("kdialog", "--getexistingdirectory", ".", "--title", "Select search directory")
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}
	return "", fmt.Errorf("no supported dialog tool found (zenity or kdialog)")
}
