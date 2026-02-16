# Swift Integration Guide

mac-cleaner v1.1 exposes an IPC server via Unix domain socket with an NDJSON (newline-delimited JSON) protocol. This guide shows how to connect from a Swift macOS app.

## Starting the Server

```bash
mac-cleaner serve --socket /tmp/mac-cleaner.sock
```

The server listens on the specified Unix domain socket. It handles one connection at a time, cleans up stale sockets on startup, and shuts down gracefully on SIGINT/SIGTERM.

## Protocol

Each message is a single JSON object terminated by `\n`. The client sends **requests**, the server responds with **responses**.

### Request Format

```json
{"id": "unique-id", "method": "ping", "params": {}}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Client-assigned identifier, echoed in all responses |
| `method` | string | One of: `ping`, `scan`, `cleanup`, `categories`, `shutdown` |
| `params` | object | Method-specific parameters (optional) |

### Response Format

```json
{"id": "unique-id", "type": "result", "result": {...}}
{"id": "unique-id", "type": "progress", "result": {...}}
{"id": "unique-id", "type": "error", "error": "message"}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Echoes the request ID |
| `type` | string | `result` (final), `progress` (streaming), or `error` |
| `result` | object | Method-specific data (on `result` and `progress` types) |
| `error` | string | Error description (on `error` type) |

## Methods

### `ping`

Health check. No params.

```json
→ {"id":"1","method":"ping"}
← {"id":"1","type":"result","result":{"status":"ok","version":"1.1.0"}}
```

### `categories`

List available scanner groups. No params.

```json
→ {"id":"2","method":"categories"}
← {"id":"2","type":"result","result":{"scanners":[
    {"id":"system","label":"System Caches"},
    {"id":"browser","label":"Browser Data"},
    {"id":"developer","label":"Developer Caches"},
    {"id":"appleftovers","label":"App Leftovers"},
    {"id":"creative","label":"Creative App Caches"},
    {"id":"messaging","label":"Messaging App Caches"}
  ]}}
```

### `scan`

Run a full scan with streaming progress. Optional `skip` param filters category IDs.

```json
→ {"id":"3","method":"scan","params":{"skip":["dev-docker"]}}
← {"id":"3","type":"progress","result":{"event":"scanner_start","scanner_id":"system","label":"System Caches"}}
← {"id":"3","type":"progress","result":{"event":"scanner_done","scanner_id":"system","label":"System Caches"}}
← {"id":"3","type":"progress","result":{"event":"scanner_start","scanner_id":"browser","label":"Browser Data"}}
...
← {"id":"3","type":"result","result":{"categories":[...],"total_size":12345678}}
```

### `cleanup`

Clean up scan results. Must follow a `scan` call (replay protection). Optional `categories` param filters which category IDs to clean.

```json
→ {"id":"4","method":"cleanup","params":{"categories":["system-caches","system-logs"]}}
← {"id":"4","type":"progress","result":{"event":"category_start","category":"User App Caches","current":1,"total":10}}
← {"id":"4","type":"progress","result":{"event":"entry_progress","category":"User App Caches","entry_path":"/Users/...","current":1,"total":10}}
...
← {"id":"4","type":"result","result":{"removed":8,"failed":2,"bytes_freed":5000000,"errors":["..."]}}
```

### `shutdown`

Gracefully shut down the server.

```json
→ {"id":"5","method":"shutdown"}
← {"id":"5","type":"result","result":{"status":"shutting_down"}}
```

## Swift Codable Types

```swift
import Foundation

// MARK: - Request

struct MCRequest: Codable {
    let id: String
    let method: String
    var params: AnyCodable?
}

struct ScanParams: Codable {
    var skip: [String]?
}

struct CleanupParams: Codable {
    var categories: [String]?
}

// MARK: - Response

struct MCResponse: Codable {
    let id: String
    let type: ResponseType
    var result: AnyCodable?
    var error: String?

    enum ResponseType: String, Codable {
        case result
        case progress
        case error
    }
}

// MARK: - Result Types

struct PingResult: Codable {
    let status: String
    let version: String
}

struct ScanEntry: Codable {
    let path: String
    let description: String
    let size: Int64
    let riskLevel: String

    enum CodingKeys: String, CodingKey {
        case path, description, size
        case riskLevel = "risk_level"
    }
}

struct CategoryResult: Codable {
    let category: String
    let description: String
    let entries: [ScanEntry]
    let totalSize: Int64

    enum CodingKeys: String, CodingKey {
        case category, description, entries
        case totalSize = "total_size"
    }
}

struct ScanResult: Codable {
    let categories: [CategoryResult]
    let totalSize: Int64

    enum CodingKeys: String, CodingKey {
        case categories
        case totalSize = "total_size"
    }
}

struct CleanupResult: Codable {
    let removed: Int
    let failed: Int
    let bytesFreed: Int64
    var errors: [String]?

    enum CodingKeys: String, CodingKey {
        case removed, failed, errors
        case bytesFreed = "bytes_freed"
    }
}

// MARK: - Progress Types

struct ScanProgress: Codable {
    let event: String  // "scanner_start", "scanner_done", "scanner_error"
    let scannerID: String
    let label: String
    var error: String?

    enum CodingKeys: String, CodingKey {
        case event, label, error
        case scannerID = "scanner_id"
    }
}

struct CleanupProgress: Codable {
    let event: String  // "category_start", "entry_progress"
    let category: String
    var entryPath: String?
    let current: Int
    let total: Int

    enum CodingKeys: String, CodingKey {
        case event, category, current, total
        case entryPath = "entry_path"
    }
}

struct CategoriesResult: Codable {
    let scanners: [ScannerInfo]
}

struct ScannerInfo: Codable {
    let id: String
    let label: String
}
```

## Swift Connection Example (Network.framework)

```swift
import Network

class MacCleanerClient {
    private var connection: NWConnection?
    private var requestID = 0

    func connect(socketPath: String) {
        let endpoint = NWEndpoint.unix(path: socketPath)
        let params = NWParameters()
        params.defaultProtocolStack.transportProtocol = NWProtocolTCP.Options()

        connection = NWConnection(to: endpoint, using: params)

        connection?.stateUpdateHandler = { state in
            switch state {
            case .ready:
                print("Connected to mac-cleaner")
                self.startReceiving()
            case .failed(let error):
                print("Connection failed: \(error)")
            default:
                break
            }
        }

        connection?.start(queue: .main)
    }

    func send(method: String, params: Codable? = nil) {
        requestID += 1
        let request = MCRequest(
            id: String(requestID),
            method: method,
            params: params.map { AnyCodable($0) }
        )

        guard let data = try? JSONEncoder().encode(request),
              var message = String(data: data, encoding: .utf8) else {
            return
        }
        message += "\n"

        connection?.send(
            content: message.data(using: .utf8),
            completion: .contentProcessed { error in
                if let error { print("Send error: \(error)") }
            }
        )
    }

    private func startReceiving() {
        connection?.receive(minimumIncompleteLength: 1, maximumLength: 65536) {
            [weak self] content, _, isComplete, error in

            if let data = content, let text = String(data: data, encoding: .utf8) {
                // Split on newlines — each line is a complete JSON message
                for line in text.split(separator: "\n") {
                    self?.handleMessage(String(line))
                }
            }

            if !isComplete {
                self?.startReceiving()
            }
        }
    }

    private func handleMessage(_ json: String) {
        guard let data = json.data(using: .utf8),
              let response = try? JSONDecoder().decode(MCResponse.self, from: data) else {
            return
        }

        switch response.type {
        case .progress:
            // Handle streaming progress updates
            print("Progress: \(response.result ?? AnyCodable("unknown"))")
        case .result:
            // Handle final result
            print("Result: \(response.result ?? AnyCodable("empty"))")
        case .error:
            print("Error: \(response.error ?? "unknown")")
        }
    }

    func disconnect() {
        connection?.cancel()
        connection = nil
    }
}
```

## Lifecycle Management

The recommended pattern for managing the mac-cleaner server process:

1. **Launch:** Use `Process` to start `mac-cleaner serve --socket <path>`
2. **Connect:** Wait for socket file to appear, then connect via `NWConnection`
3. **Health check:** Send `ping` to verify the server is responsive
4. **Scan → Review → Cleanup:** Full workflow with streaming progress
5. **Shutdown:** Send `shutdown` method or terminate the process

```swift
let process = Process()
process.executableURL = URL(fileURLWithPath: "/usr/local/bin/mac-cleaner")
process.arguments = ["serve", "--socket", socketPath]
try process.run()
```

## Error Handling

- **Concurrent operations:** Only one scan or cleanup can run at a time. Additional requests get an error response.
- **Cleanup without scan:** The server requires a scan before cleanup (replay protection). After cleanup, scan results are cleared.
- **Client disconnect:** If the client disconnects during a scan or cleanup, the server stops streaming and cleans up gracefully.
- **Idle timeout:** Connections idle for more than 5 minutes are automatically closed.
- **Stale sockets:** On startup, the server detects and removes stale socket files from crashed instances.

## Testing with socat

You can test the server manually using `socat`:

```bash
# Start server
mac-cleaner serve --socket /tmp/mc.sock &

# Connect and send ping
echo '{"id":"1","method":"ping"}' | socat - UNIX-CONNECT:/tmp/mc.sock

# Interactive session
socat READLINE UNIX-CONNECT:/tmp/mc.sock
# Then type JSON requests line by line
```
