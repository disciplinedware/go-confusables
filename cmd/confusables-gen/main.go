// confusables-gen is a tool to generate confusables data from unicode.org.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	latestConfusablesURL    = "https://unicode.org/Public/security/latest/confusables.txt"
	versionedConfusablesURL = "https://unicode.org/Public/security/%s/confusables.txt"
)

type Mapping struct {
	Source     int    `json:"source"`
	Target     []int  `json:"target"`
	SourceName string `json:"source_name"`
	TargetName string `json:"target_name"`
}

type DataFile struct {
	UnicodeVersion string    `json:"unicode_version"`
	GeneratedAt    time.Time `json:"generated_at"`
	SourceURL      string    `json:"source_url"`
	SourceDate     string    `json:"source_date"`
	TotalMappings  int       `json:"total_mappings"`
	Mappings       []Mapping `json:"mappings"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("confusables-gen", flag.ContinueOnError)
	version := fs.String("version", "latest", "Unicode version to download")
	input := fs.String("input", "", "Path to local confusables.txt (offline mode)")
	output := fs.String("output", "data/confusables.json", "Output JSON path")
	genAt := fs.String("generated-at", "", "Override generated_at timestamp (RFC3339)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var reader io.ReadCloser
	var sourceURL string
	var err error

	v := *version
	if *input != "" {
		fmt.Printf("Reading from local file: %s\n", *input)
		reader, err = os.Open(*input)
		if err != nil {
			return fmt.Errorf("failed to open input file: %v", err)
		}
		sourceURL = "local file: " + *input
	} else {
		// Normalize version: 16.0 -> 16.0.0 (if strictly needed by URL, but usually latest works)
		if v != "latest" && strings.Count(v, ".") == 1 {
			v += ".0"
		}
		url := latestConfusablesURL
		if v != "latest" {
			url = fmt.Sprintf(versionedConfusablesURL, v)
		}
		fmt.Printf("Downloading: %s\n", url)
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return fmt.Errorf("failed to download confusables: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return fmt.Errorf("bad status: %s", resp.Status)
		}
		reader = resp.Body
		sourceURL = url
	}
	defer func() { _ = reader.Close() }()

	dataFile, err := parseConfusables(reader, sourceURL, v)
	if err != nil {
		return fmt.Errorf("failed to parse: %v", err)
	}

	if *genAt != "" {
		t, err := time.Parse(time.RFC3339, *genAt)
		if err != nil {
			return fmt.Errorf("failed to parse generated-at: %v", err)
		}
		dataFile.GeneratedAt = t
	}

	jsonData, err := json.MarshalIndent(dataFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %v", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(*output)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	if err := os.WriteFile(*output, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write output: %v", err)
	}

	fmt.Printf("Generated %s\n", *output)
	fmt.Printf("Total mappings: %d\n", dataFile.TotalMappings)
	fmt.Printf("Unicode version: %s\n", dataFile.UnicodeVersion)
	return nil
}

func parseConfusables(r io.Reader, sourceURL, version string) (*DataFile, error) {
	dataFile := &DataFile{
		UnicodeVersion: version,
		GeneratedAt:    time.Now().UTC(),
		SourceURL:      sourceURL,
	}

	seenSources := make(map[int]bool)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse header if available
		if strings.HasPrefix(line, "#") {
			if strings.Contains(line, "Version:") && dataFile.UnicodeVersion == "latest" {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					dataFile.UnicodeVersion = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(line, "Date:") && dataFile.SourceDate == "" {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					dataFile.SourceDate = strings.TrimSpace(parts[1])
				}
			}
			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "#")
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed line (missing comment): %q", line)
		}

		dataPart := parts[0]
		commentPart := parts[1]

		fields := strings.Split(dataPart, ";")
		if len(fields) < 2 {
			return nil, fmt.Errorf("malformed line (missing fields): %q", line)
		}

		sourceHex := strings.TrimSpace(fields[0])
		targetHexes := strings.Fields(fields[1])

		sourceRune, err := strconv.ParseInt(sourceHex, 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source hex %q: %v", sourceHex, err)
		}

		// Basic Unicode scalar value check (roughly)
		if sourceRune < 0 || sourceRune > 0x10FFFF || (sourceRune >= 0xD800 && sourceRune <= 0xDFFF) {
			return nil, fmt.Errorf("invalid unicode source codepoint: %04X", sourceRune)
		}

		if seenSources[int(sourceRune)] {
			return nil, fmt.Errorf("duplicate source: %04X", sourceRune)
		}

		var targetRunes []int
		for _, hex := range targetHexes {
			tr, err := strconv.ParseInt(hex, 16, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid target hex %q for source %04X: %v", hex, sourceRune, err)
			}
			if tr < 0 || tr > 0x10FFFF || (tr >= 0xD800 && tr <= 0xDFFF) {
				return nil, fmt.Errorf("invalid unicode target codepoint: %04X", tr)
			}
			targetRunes = append(targetRunes, int(tr))
		}

		if len(targetRunes) == 0 {
			return nil, fmt.Errorf("empty target for source %04X", sourceRune)
		}

		// Parse names from comment
		namesPart := commentPart
		if idx := strings.LastIndex(commentPart, ")"); idx != -1 {
			namesPart = commentPart[idx+1:]
		}

		nameFields := strings.Split(namesPart, "→")
		sourceName := ""
		targetName := ""
		if len(nameFields) == 2 {
			sourceName = strings.TrimSpace(nameFields[0])
			targetName = strings.TrimSpace(nameFields[1])
		}

		dataFile.Mappings = append(dataFile.Mappings, Mapping{
			Source:     int(sourceRune),
			Target:     targetRunes,
			SourceName: sourceName,
			TargetName: targetName,
		})
		seenSources[int(sourceRune)] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	dataFile.TotalMappings = len(dataFile.Mappings)
	return dataFile, nil
}
