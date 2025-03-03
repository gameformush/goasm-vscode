package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime/pprof"
	"strings"

	"gioui.org/app"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func main() {
	cpuprofile := flag.String("cpuprofile", "", "enable cpu profiling")
	textSize := flag.Int("text-size", 12, "default font size")
	filter := flag.String("filter", "", "filter the functions by regexp")
	watch := flag.Bool("watch", false, "auto reload executable")
	context := flag.Int("context", 3, "source line context")
	font := flag.String("font", "", "user font")
	darkMode := flag.Bool("dark", false, "use dark theme")

	// HTTP server/client options
	serverMode := flag.Bool("server", false, "run in server mode (HTTP API only)")
	clientMode := flag.Bool("client", false, "run in client mode (connect to HTTP server)")
	serverAddr := flag.String("addr", "localhost:8080", "HTTP server address (format: host:port)")

	workInProgressWASM = os.Getenv("LENSM_EXPERIMENT_WASM") != ""

	flag.Parse()
	exePath := flag.Arg(0)

	if exePath == "" {
		fmt.Fprintln(os.Stderr, "lensm <exePath>")
		flag.Usage()
		os.Exit(1)
	}

	// Debug code removed

	// Check for incompatible modes
	if *serverMode && *clientMode {
		fmt.Fprintln(os.Stderr, "Error: Cannot use both -server and -client modes at the same time")
		os.Exit(1)
	}

	// Start in server mode if requested
	if *serverMode {
		fmt.Printf("Starting lensm in server mode on %s\n", *serverAddr)
		StartServer(*serverAddr, *context)
		return
	}

	// Set the server URL if in client mode
	var serverURL string
	if *clientMode {
		// Check if the address starts with http://
		if !strings.HasPrefix(*serverAddr, "http://") {
			serverURL = "http://" + *serverAddr
		} else {
			serverURL = *serverAddr
		}
		fmt.Printf("Running in client mode, connecting to %s\n", serverURL)
	}

	windows := &Windows{}

	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.WithCollection(LoadFonts(*font)))
	theme.TextSize = unit.Sp(*textSize)

	// Apply dark theme if requested
	if *darkMode {
		// Set global dark mode state
		isDarkMode = true

		// Set global colors for widgets
		secondaryBackground = darkSecondaryBackground
		splitterColor = darkSplitterColor

		// Set theme colors
		theme.Bg = color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF}
		theme.Fg = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
		theme.ContrastBg = color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xFF}
		theme.ContrastFg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}

	ui := NewExeUI(windows, theme)
	ui.Config = FileUIConfig{
		Path:      exePath,
		Watch:     *watch,
		Context:   *context,
		ServerURL: serverURL,
	}
	ui.Funcs.SetFilter(*filter)

	windows.Open("lensm", image.Pt(1400, 900), ui.Run)

	go func() {
		profile(*cpuprofile, windows.Wait)
		os.Exit(0)
	}()

	// This starts Gio main.
	app.Main()
}

var (
	secondaryBackground = color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	splitterColor       = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}

	// Light theme colors
	lightSecondaryBackground = color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	lightSplitterColor       = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}

	// Dark theme colors
	darkSecondaryBackground = color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF}
	darkSplitterColor       = color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xFF}

	// Default to light theme, will be set in main() based on the -dark flag

	// Is dark mode enabled
	isDarkMode = false
)

func profile(cpuprofile string, fn func()) {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	fn()
}
