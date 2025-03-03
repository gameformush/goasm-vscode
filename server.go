package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"loov.dev/lensm/internal/disasm"
	"loov.dev/lensm/internal/goobj"
	"loov.dev/lensm/internal/wasmobj"
)

// Server handles HTTP requests for disassembly operations
type Server struct {
	// activeFiles maps file paths to loaded disasm.File instances
	activeFiles      map[string]disasm.File
	activeFilesMutex sync.RWMutex

	// Options for disassembly
	options disasm.Options
}

// NewServer creates a new HTTP server for disassembly operations
func NewServer(context int) *Server {
	return &Server{
		activeFiles: make(map[string]disasm.File),
		options: disasm.Options{
			Context: context,
		},
	}
}

// StartServer starts the HTTP server on the specified address
func StartServer(addr string, context int) {
	server := NewServer(context)

	// Create a new router using Gorilla Mux
	r := mux.NewRouter()

	// Set up middleware
	r.Use(loggingMiddleware)

	// API routes
	r.HandleFunc("/api/files", server.handleFiles).Methods("GET", "POST")
	r.HandleFunc("/api/files/{path:.+}", server.handleFileOperations).Methods("DELETE")
	r.HandleFunc("/api/functions", server.handleFunctions).Methods("GET")
	r.HandleFunc("/api/functions/{name:.+}", server.handleFunctionOperations).Methods("GET")

	// Create a CORS handler with the rs/cors package
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*", "vscode-webview://*"}, // All origins including VS Code webviews
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Accept", "Authorization", "X-Requested-With", "Origin"},
		AllowCredentials: true,
		MaxAge:           86400, // Maximum value not ignored by any major browser (1 day)
		Debug:            true,  // Enable debugging for troubleshooting
		// Set the Vary header to tell browsers to cache responses based on Origin header
		OptionsPassthrough: false,
		OptionsSuccessStatus: http.StatusOK,
	})

	// Wrap the router with the CORS handler
	handler := c.Handler(r)

	// Start the server
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

// loggingMiddleware logs all requests with their paths and methods
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// handleFiles handles operations on the collection of files
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	// OPTIONS requests should be handled before this function is called
	switch r.Method {
	case http.MethodPost:
		// Load a new file
		var req struct {
			Path string `json:"path"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		if req.Path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		// Check if we already have this file loaded
		s.activeFilesMutex.RLock()
		_, exists := s.activeFiles[req.Path]
		s.activeFilesMutex.RUnlock()

		if exists {
			// File already loaded
			w.WriteHeader(http.StatusOK)
			return
		}

		// Load the file
		var file disasm.File
		var err error

		if workInProgressWASM && isWasmFile(req.Path) {
			file, err = wasmobj.Load(req.Path)
		} else {
			file, err = goobj.Load(req.Path)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to load file: %v", err), http.StatusInternalServerError)
			return
		}

		// Store the file
		s.activeFilesMutex.Lock()
		s.activeFiles[req.Path] = file
		s.activeFilesMutex.Unlock()

		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		// List all loaded files
		s.activeFilesMutex.RLock()
		files := make([]string, 0, len(s.activeFiles))
		for path := range s.activeFiles {
			files = append(files, path)
		}
		s.activeFilesMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": files,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFileOperations handles operations on a specific file
func (s *Server) handleFileOperations(w http.ResponseWriter, r *http.Request) {
	// OPTIONS requests should be handled before this function is called

	// Extract the file path from the URL using Gorilla Mux vars
	vars := mux.Vars(r)
	path := vars["path"]
	if path == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	// Only handle DELETE method (others are configured in the router)
	// Close and remove a file
	s.activeFilesMutex.Lock()
	file, exists := s.activeFiles[path]
	if exists {
		delete(s.activeFiles, path)
	}
	s.activeFilesMutex.Unlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if err := file.Close(); err != nil {
		log.Printf("Error closing file %s: %v", path, err)
	}

	w.WriteHeader(http.StatusOK)
}

// handleFunctions handles operations on the collection of functions in a file
func (s *Server) handleFunctions(w http.ResponseWriter, r *http.Request) {
	// OPTIONS requests should be handled before this function is called

	// Get query parameters
	query := r.URL.Query()
	path := query.Get("file")
	filter := query.Get("filter")

	if path == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	// Get the file
	s.activeFilesMutex.RLock()
	file, exists := s.activeFiles[path]
	s.activeFilesMutex.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Get all functions
	funcs := file.Funcs()

	// Filter functions if a filter is provided
	var filteredFuncs []FunctionInfo
	if filter != "" {
		rx, err := regexp.Compile(filter)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid filter regex: %v", err), http.StatusBadRequest)
			return
		}

		for _, fn := range funcs {
			if rx.MatchString(fn.Name()) {
				filteredFuncs = append(filteredFuncs, FunctionInfo{
					Name: fn.Name(),
				})
			}
		}
	} else {
		// No filter, return all functions
		filteredFuncs = make([]FunctionInfo, len(funcs))
		for i, fn := range funcs {
			filteredFuncs[i] = FunctionInfo{
				Name: fn.Name(),
			}
		}
	}

	// Set content type and encode the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"functions": filteredFuncs,
	})
}

// handleFunctionOperations handles operations on a specific function
func (s *Server) handleFunctionOperations(w http.ResponseWriter, r *http.Request) {
	// OPTIONS requests should be handled before this function is called

	// Extract the function name from the URL using Gorilla Mux vars
	vars := mux.Vars(r)
	functionName := vars["name"]
	if functionName == "" {
		http.Error(w, "Function name is required", http.StatusBadRequest)
		return
	}

	// Get query parameters
	query := r.URL.Query()
	path := query.Get("file")
	contextStr := query.Get("context")

	if path == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	// Get the file
	s.activeFilesMutex.RLock()
	file, exists := s.activeFiles[path]
	s.activeFilesMutex.RUnlock()

	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Find the function
	funcs := file.Funcs()
	var targetFunc disasm.Func
	for _, fn := range funcs {
		if fn.Name() == functionName {
			targetFunc = fn
			break
		}
	}

	if targetFunc == nil {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	// Set context if provided
	options := s.options
	if contextStr != "" {
		context, err := strconv.Atoi(contextStr)
		if err != nil {
			http.Error(w, "Invalid context value", http.StatusBadRequest)
			return
		}
		options.Context = context
	}

	// Load the function code
	code := targetFunc.Load(options)
	if code == nil {
		http.Error(w, "Failed to load function code", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := CodeResponse{
		Name:         code.Name,
		File:         code.File,
		Instructions: make([]InstructionInfo, len(code.Insts)),
		Sources:      make([]SourceInfo, len(code.Source)),
		MaxJump:      code.MaxJump,
	}

	// Convert instructions
	for i, inst := range code.Insts {
		response.Instructions[i] = InstructionInfo{
			PC:        inst.PC,
			Text:      inst.Text,
			File:      inst.File,
			Line:      inst.Line,
			RefPC:     inst.RefPC,
			RefOffset: inst.RefOffset,
			RefStack:  inst.RefStack,
			Call:      inst.Call,
		}
	}

	// Convert sources
	for i, src := range code.Source {
		sourceInfo := SourceInfo{
			File:   src.File,
			Blocks: make([]SourceBlockInfo, len(src.Blocks)),
		}

		for j, block := range src.Blocks {
			blockInfo := SourceBlockInfo{
				From:    block.From,
				To:      block.To,
				Lines:   block.Lines,
				Related: make([][]LineRangeInfo, len(block.Related)),
			}

			for k, relatedRanges := range block.Related {
				blockInfo.Related[k] = make([]LineRangeInfo, len(relatedRanges))
				for l, rng := range relatedRanges {
					blockInfo.Related[k][l] = LineRangeInfo{
						From: rng.From,
						To:   rng.To,
					}
				}
			}

			sourceInfo.Blocks[j] = blockInfo
		}

		response.Sources[i] = sourceInfo
	}

	// Set content type and encode the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Response types for the API

// FunctionInfo represents a function in an object file
type FunctionInfo struct {
	Name string `json:"name"`
}

// CodeResponse represents the disassembled code of a function
type CodeResponse struct {
	Name         string            `json:"name"`
	File         string            `json:"file"`
	Instructions []InstructionInfo `json:"instructions"`
	Sources      []SourceInfo      `json:"sources"`
	MaxJump      int               `json:"maxJump"`
}

// InstructionInfo represents a single assembly instruction
type InstructionInfo struct {
	PC        uint64 `json:"pc"`
	Text      string `json:"text"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	RefPC     uint64 `json:"refPc"`
	RefOffset int    `json:"refOffset"`
	RefStack  int    `json:"refStack"`
	Call      string `json:"call"`
}

// SourceInfo represents source code from a single file
type SourceInfo struct {
	File   string            `json:"file"`
	Blocks []SourceBlockInfo `json:"blocks"`
}

// SourceBlockInfo represents a single block of source code
type SourceBlockInfo struct {
	From    int               `json:"from"`
	To      int               `json:"to"`
	Lines   []string          `json:"lines"`
	Related [][]LineRangeInfo `json:"related"`
}

// LineRangeInfo represents a range of lines
type LineRangeInfo struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Helper function to check if a file is a WebAssembly file
func isWasmFile(path string) bool {
	return regexp.MustCompile(`\.wasm$`).MatchString(path)
}
