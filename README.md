# Go Assembly Viewer - VSCode Extension

A VSCode extension for viewing Go assembly code directly from your editor. This extension leverages the [lensm](https://github.com/loov/lensm) project to provide a seamless interface for examining Go assembly alongside your source code.

![Go Assembly Viewer in action](screenshot.gif)

## Features

- View disassembled Go code with corresponding source lines
- Browse functions with search/filter capability
- Interactive assembly code viewer
- Side-by-side view of Go source and assembly code

## Requirements

- Go installed on your system
- lensm server running (see installation instructions below)

## Installation

### 1. Install the extension

Install this extension from the VS Code Marketplace or using the Extensions view in VS Code.

### 2. Install and run lensm server

The extension requires the lensm server to be running. To install and run lensm:

```bash
# Install lensm
go install github.com/loov/lensm@latest

# Run lensm in server mode
lensm -server -addr localhost:8080
```

## Usage

1. Build your Go project with debug information preserved
2. Run the "Go Assembly: Show Assembly View" command from the Command Palette
3. Browse and search functions to view their assembly code

## Extension Settings

This extension contributes the following settings:

* `goasm.serverUrl`: The URL of the lensm server (default: "http://localhost:8080")

## Known Issues

- The lensm server must be running for the extension to work
- Large binaries may take some time to load

## Release Notes

### 0.0.1

Initial release with basic Go assembly viewing capabilities.

---

**Note**: This extension is in early development. Feedback and contributions are welcome!