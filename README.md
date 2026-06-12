# DeepFind

[![Release](https://github.com/sekarbala123/deepfind/actions/workflows/release.yml/badge.svg)](https://github.com/sekarbala123/deepfind/actions/workflows/release.yml)

**DeepFind** is a high-performance, cross-platform file and archive search engine written in Go. It provides both a blazing-fast Command-Line Interface (CLI) and a modern, interactive web-based dashboard UI.

## Features

- ⚡ **Concurrent Scan**: Walks the file system trees concurrently using light-weight goroutines for rapid execution.
- 📦 **Embedded Assets**: Web dashboard static assets (HTML, CSS, JavaScript) are compiled directly into the Go executable using `go:embed`. No installation directories required.
- 📂 **Native Directory Selector**: Clicking the browser folder button in the Web UI triggers the local host operating system's native folder selection dialog (AppleScript on macOS, PowerShell on Windows, Zenity/KDialog on Linux) for a seamless native desktop experience.
- 🗜️ **Dynamic Archive Searching**:
  - Scans inside Zip-based (`.zip`, `.jar`, `.war`) and Tar-based (`.tar`, `.tgz`, `.gz`, `.tar.gz`) archives.
  - Allows users to select, deselect, or dynamically add custom archive formats (e.g. mapping `.apk` to Zip decompression) directly in the UI dashboard.
  - Automatically identifies gzip-compressed tarballs by checking file header magic bytes (`0x1f 0x8b`), remaining extension-agnostic.
- 🔍 **Flexible Search Syntax**: Supports substring matching, wildcard globs, or complete Regular Expressions for both file names and file content.
- 🎨 **Premium Glassmorphic UI**: Real-time result updates streamed via Server-Sent Events (SSE), search metric charts, live results filtering, and path clipboard copy buttons.

---

## Getting Started

### Installation

Download pre-built binaries for your platform from the [Releases](https://github.com/sekarbala123/deepfind/releases) tab, or build from source (see below).

### Interactive Web UI Mode

Launch the local web server and open the browser dashboard:
```bash
# Start on default port 8080 and auto-open dashboard in browser
./deepfind -ui

# Start on a custom port
./deepfind -ui -port 9000
```

### Command-Line Search Mode

You can run queries directly from your terminal:
```bash
# General usage
deepfind -name <filename-pattern> -content <text-pattern> [options]

# Example: Search for JSON files containing the word "version"
deepfind -name "*.json" -content "version" -path "/path/to/project"

# Options list
Options:
  -path string
        Directory path to start searching from (default ".")
  -name string
        Filename pattern (glob wildcards like *.json or regex)
  -content string
        Text content to search for inside files
  -archive
        Search inside ZIP, JAR, WAR, and TAR archives (default false)
  -regex
        Interpret filename and content patterns as regular expressions (default false)
  -ignore-case
        Perform case-insensitive matching (default true)
```

---

## Building from Source

Ensure you have [Go](https://go.dev/dl/) installed (version 1.16 or higher).

### Compilation Targets

A standard `Makefile` is provided for compilation task execution:

```bash
# Build the native binary for your host OS
make build

# Run unit tests
make test

# Cross-compile stand-alone binaries for macOS (Intel & Apple Silicon), Windows, and Linux
make build-all
```

The compiled binaries will be output into the `dist/` directory.

---

## Repository CI/CD Release Flow

This repository includes a GitHub Actions CI/CD release workflow configured in `.github/workflows/release.yml`. 

Whenever a new version tag is pushed:
1. All unit tests are run to guarantee stability.
2. Multiplatform binaries are built for macOS, Windows, and Linux.
3. A GitHub Release is created automatically with the compiled assets attached.

### Triggering a Release
To publish a new version:
```bash
# 1. Create and annotate a git tag
git tag -a v1.0.0 -m "Release version 1.0.0"

# 2. Push tag to remote
git push origin v1.0.0
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
