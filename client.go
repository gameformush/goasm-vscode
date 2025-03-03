package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"loov.dev/lensm/internal/disasm"
)

// Client handles communication with the lensm HTTP server
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new client for the lensm HTTP server
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoadFile loads a binary file for disassembly
func (c *Client) LoadFile(path string) error {
	reqBody := struct {
		Path string `json:"path"`
	}{
		Path: path,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create a custom request to add headers
	req, err := http.NewRequest(
		"POST",
		c.baseURL+"/api/files",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (status %d): %s", resp.StatusCode, body)
	}

	return nil
}

// GetFunctions retrieves functions from a loaded file
func (c *Client) GetFunctions(path string, filter string) ([]FunctionInfo, error) {
	params := url.Values{}
	params.Add("file", path)
	if filter != "" {
		params.Add("filter", filter)
	}

	resp, err := c.httpClient.Get(c.baseURL + "/api/functions?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", body)
	}

	var result struct {
		Functions []FunctionInfo `json:"functions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return result.Functions, nil
}

// GetFunctionCode retrieves the disassembled code for a specific function
func (c *Client) GetFunctionCode(path string, functionName string, context int) (*disasm.Code, error) {
	params := url.Values{}
	params.Add("file", path)
	if context > 0 {
		params.Add("context", fmt.Sprintf("%d", context))
	}

	// URL encode the function name
	escapedName := url.PathEscape(functionName)

	resp, err := c.httpClient.Get(c.baseURL + "/api/functions/" + escapedName + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", body)
	}

	var result CodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Convert the response to a disasm.Code object
	code := &disasm.Code{
		Name:    result.Name,
		File:    result.File,
		MaxJump: result.MaxJump,
		Insts:   make([]disasm.Inst, len(result.Instructions)),
		Source:  make([]disasm.Source, len(result.Sources)),
	}

	// Convert instructions
	for i, inst := range result.Instructions {
		code.Insts[i] = disasm.Inst{
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
	for i, src := range result.Sources {
		source := disasm.Source{
			File:   src.File,
			Blocks: make([]disasm.SourceBlock, len(src.Blocks)),
		}

		for j, block := range src.Blocks {
			sourceBlock := disasm.SourceBlock{
				LineRange: disasm.LineRange{
					From: block.From,
					To:   block.To,
				},
				Lines:   block.Lines,
				Related: make([][]disasm.LineRange, len(block.Related)),
			}

			for k, relatedRanges := range block.Related {
				sourceBlock.Related[k] = make([]disasm.LineRange, len(relatedRanges))
				for l, rng := range relatedRanges {
					sourceBlock.Related[k][l] = disasm.LineRange{
						From: rng.From,
						To:   rng.To,
					}
				}
			}

			source.Blocks[j] = sourceBlock
		}

		code.Source[i] = source
	}

	return code, nil
}

// NetworkFile implements the disasm.File interface for remote files
type NetworkFile struct {
	client  *Client
	path    string
	filter  *regexp.Regexp
	funcs   []disasm.Func
	funcMap map[string]disasm.Func
}

// NetworkFunc implements the disasm.Func interface for remote functions
type NetworkFunc struct {
	file *NetworkFile
	name string
}

// Ensure interfaces are implemented
var _ disasm.File = (*NetworkFile)(nil)
var _ disasm.Func = (*NetworkFunc)(nil)

// NewNetworkFile creates a new NetworkFile
func NewNetworkFile(client *Client, path string) (*NetworkFile, error) {
	file := &NetworkFile{
		client:  client,
		path:    path,
		funcMap: make(map[string]disasm.Func),
	}

	if err := client.LoadFile(path); err != nil {
		return nil, err
	}

	// Get all functions
	functions, err := client.GetFunctions(path, "")
	if err != nil {
		return nil, err
	}

	// Create function objects
	file.funcs = make([]disasm.Func, len(functions))
	for i, fn := range functions {
		netFunc := &NetworkFunc{
			file: file,
			name: fn.Name,
		}
		file.funcs[i] = netFunc
		file.funcMap[fn.Name] = netFunc
	}

	return file, nil
}

// Close implements disasm.File.Close
func (f *NetworkFile) Close() error {
	// Make a DELETE request to clean up resources on the server
	encodedPath := url.PathEscape(f.path)
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/files/%s", f.client.baseURL, encodedPath), nil)
	if err != nil {
		return err
	}

	resp, err := f.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, body)
	}

	return nil
}

// Funcs implements disasm.File.Funcs
func (f *NetworkFile) Funcs() []disasm.Func {
	return f.funcs
}

// Name implements disasm.Func.Name
func (f *NetworkFunc) Name() string {
	return f.name
}

// Load implements disasm.Func.Load
func (f *NetworkFunc) Load(opt disasm.Options) *disasm.Code {
	code, err := f.file.client.GetFunctionCode(f.file.path, f.name, opt.Context)
	if err != nil {
		// Log error but don't fail
		fmt.Printf("Error loading function %s: %v\n", f.name, err)
		return nil
	}
	return code
}

// LoadNetworkFile loads a file using the HTTP client
func LoadNetworkFile(serverURL, filePath string) (disasm.File, error) {
	client := NewClient(serverURL)
	return NewNetworkFile(client, filePath)
}
