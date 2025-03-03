# Lensm HTTP API Documentation

Lensm provides an HTTP API for disassembly operations. This document describes the available endpoints, their parameters, and example request/response patterns.

The API is built using the Gorilla Mux router, which provides powerful routing capabilities, URL parameter extraction, and middleware support.

## API Endpoints

### File Operations

#### Load a File

Loads a binary file for disassembly.

```
POST /api/files
```

**Request Parameters**

| Parameter | Type   | Required | Description                    |
|-----------|--------|----------|--------------------------------|
| path      | string | Yes      | Path to the executable file    |

**Request Example**

```json
{
  "path": "/path/to/executable"
}
```

**Response**

- HTTP 201 Created: File loaded successfully
- HTTP 200 OK: File already loaded
- HTTP 400 Bad Request: Invalid request
- HTTP 500 Internal Server Error: Failed to load file

#### List Loaded Files

Lists all currently loaded files.

```
GET /api/files
```

**Response Example**

```json
{
  "files": [
    "/path/to/executable1",
    "/path/to/executable2"
  ]
}
```

#### Close a File

Closes a previously loaded file and frees resources.

```
DELETE /api/files/{path}
```

**Response**

- HTTP 200 OK: File closed successfully
- HTTP 404 Not Found: File not found
- HTTP 500 Internal Server Error: Failed to close file

### Function Operations

#### List Functions

Lists all functions in a loaded file, optionally filtered by a regular expression.

```
GET /api/functions?file={path}&filter={regex}
```

**Query Parameters**

| Parameter | Type   | Required | Description                   |
|-----------|--------|----------|-------------------------------|
| file      | string | Yes      | Path of the loaded file       |
| filter    | string | No       | Regex to filter function names|

**Response Example**

```json
{
  "functions": [
    {
      "name": "main.main"
    },
    {
      "name": "main.NewExeUI"
    }
  ]
}
```

**Response**

- HTTP 200 OK: Functions retrieved successfully
- HTTP 400 Bad Request: Invalid request or filter regex
- HTTP 404 Not Found: File not found
- HTTP 500 Internal Server Error: Failed to retrieve functions

#### Get Function Code

Retrieves the disassembled code of a specific function.

```
GET /api/functions/{name}?file={path}&context={number}
```

**Query Parameters**

| Parameter | Type   | Required | Description                   |
|-----------|--------|----------|-------------------------------|
| file      | string | Yes      | Path of the loaded file       |
| context   | number | No       | Number of lines of context    |

**Path Parameters**

| Parameter | Type   | Required | Description                   |
|-----------|--------|----------|-------------------------------|
| name      | string | Yes      | Name of the function          |

**Response Example**

```json
{
  "name": "main.main",
  "file": "/path/to/main.go",
  "instructions": [
    {
      "pc": 4843904,
      "text": "MOVQ FS:0xfffffff8, CX",
      "file": "/path/to/main.go",
      "line": 18,
      "refPc": 0,
      "refOffset": 0,
      "refStack": 0,
      "call": ""
    },
    ...
  ],
  "sources": [
    {
      "file": "/path/to/main.go",
      "blocks": [
        {
          "from": 15,
          "to": 25,
          "lines": [
            "func main() {",
            "    cpuprofile := flag.String(\"cpuprofile\", \"\", \"enable cpu profiling\")",
            "    ...",
            "}"
          ],
          "related": [
            [
              { "from": 0, "to": 5 }
            ]
          ]
        }
      ]
    }
  ],
  "maxJump": 2
}
```

**Response**

- HTTP 200 OK: Function code retrieved successfully
- HTTP 400 Bad Request: Invalid request
- HTTP 404 Not Found: File or function not found
- HTTP 500 Internal Server Error: Failed to retrieve function code

## Data Types

### FunctionInfo

Represents a function in a binary file.

| Field | Type   | Description                   |
|-------|--------|-------------------------------|
| name  | string | Name of the function          |

### InstructionInfo

Represents a single assembly instruction.

| Field     | Type   | Description                                 |
|-----------|--------|---------------------------------------------|
| pc        | number | Program counter                             |
| text      | string | Text representation of the instruction      |
| file      | string | Source file where this instruction came from|
| line      | number | Line number in the source file              |
| refPc     | number | Reference to another program counter        |
| refOffset | number | Reference to a relative jump                |
| refStack  | number | Depth that the jump line should be drawn at |
| call      | string | Named target (if a call instruction)        |

### SourceInfo

Represents source code from a single file.

| Field  | Type           | Description                  |
|--------|----------------|------------------------------|
| file   | string         | Source file path             |
| blocks | SourceBlockInfo[] | Source code blocks        |

### SourceBlockInfo

Represents a block of source code with related assembly instructions.

| Field   | Type            | Description                         |
|---------|-----------------|-------------------------------------|
| from    | number          | Start line number                   |
| to      | number          | End line number                     |
| lines   | string[]        | Text of each line                   |
| related | LineRangeInfo[][] | Related ranges in the instructions |

### LineRangeInfo

Represents a range of lines.

| Field | Type   | Description   |
|-------|--------|---------------|
| from  | number | Start index   |
| to    | number | End index     |

## Middleware

The API uses the following middleware components:

### Logging Middleware

All requests are logged with their methods and paths for debugging purposes.

### CORS Support

Cross-Origin Resource Sharing (CORS) is fully supported using the `rs/cors` package, which allows web clients from any origin to access the API. The following CORS settings are enabled:

- `Access-Control-Allow-Origin: *` (all origins are allowed)
- `Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Accept, Authorization, X-Requested-With`
- `Access-Control-Allow-Credentials: true`
- `Access-Control-Max-Age: 86400` (1 day)

This means you can make requests to the API from any web domain, including from websites running on different ports on localhost (e.g., `http://localhost:3000` can access the API running on `http://localhost:8080`).

## Usage Examples

### Command Line Usage

Start the server:

```bash
lensm -server -addr localhost:8080 /path/to/executable
```

Run the client:

```bash
lensm -client -addr localhost:8080 /path/to/executable
```

### API Usage Examples

#### Load a file:

```bash
curl -X POST http://localhost:8080/api/files \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/executable"}'
```

#### List functions with a filter:

```bash
curl "http://localhost:8080/api/functions?file=/path/to/executable&filter=main"
```

#### Get function code:

```bash
curl "http://localhost:8080/api/functions/main.main?file=/path/to/executable&context=3"
```