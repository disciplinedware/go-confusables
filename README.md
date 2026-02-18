# go-confusables

Unicode Confusable Characters Library for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/disciplinedware/go-confusables.svg)](https://pkg.go.dev/github.com/disciplinedware/go-confusables)
[![build](https://github.com/disciplinedware/go-confusables/actions/workflows/ci.yml/badge.svg)](https://github.com/disciplinedware/go-confusables/actions/workflows/ci.yml)

This library provides utilities to detect and normalize Unicode "confusable" characters (homoglyphs), which are often used to evade text-based security filters. It uses the authoritative data from [Unicode TR39](https://unicode.org/reports/tr39/).

## Installation

```bash
go get github.com/disciplinedware/go-confusables
```

## Features

- **Authoritative Data**: Embedded latest `confusables.txt` from unicode.org.
- **Fast**: In-memory lookups, zero network/FS calls at runtime.
- **Normalize to ASCII**: Easily replace visually similar characters with their ASCII equivalents.
- **Skeleton Algorithm**: Full implementation of the TR39 skeleton algorithm for secure string comparison.
- **CLI Tool**: Included `confusables-gen` to update data from the latest Unicode version.

## Usage

### Simple ASCII Normalization

```go
import "github.com/disciplinedware/go-confusables"

db := confusables.Default()

// Cyrillic 'а' and 'о' look like Latin 'a' and 'o'
text := "hеllо" 
normalized := db.ToASCII(text)
fmt.Println(normalized) // "hello"
```

### Secure String Comparison (Skeleton)

The skeleton algorithm is recommended for detecting spoofed strings (e.g., usernames).

```go
db := confusables.Default()

user1 := "apple"
user2 := "аррle" // Uses Cyrillic 'а' and 'р'

if db.IsConfusable(user1, user2) {
    fmt.Println("Warning: Usernames are confusable!")
}
```

### Metadata Access

Access information about the embedded Unicode dataset:

```go
fmt.Printf("Unicode Version: %s\n", db.UnicodeVersion())
fmt.Printf("Source Date: %s\n", db.SourceDate())
```

## CLI Tool: Updating Data

You can regenerate the embedded data using the provided CLI tool. For CI/reproducible builds, you can provide a deterministic timestamp:

```bash
go run ./cmd/confusables-gen --version 16.0.0 --generated-at 2026-02-18T12:00:00Z
```

## Development

A `Makefile` is provided for common development tasks:

- `make test`: Run all tests with the race detector.
- `make lint`: Run the linter.
- `make build`: Build the library and the generator tool.
- `make generate`: Update the embedded data from Unicode.org.
- `make clean`: Remove build artifacts.

## License

MIT
