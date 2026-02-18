package confusables

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"
)

// Mapping represents a single confusable mapping.
type Mapping struct {
	Source     int    `json:"source"`
	Target     []int  `json:"target"`
	SourceName string `json:"source_name"`
	TargetName string `json:"target_name"`
}

// dataFile is the internal representation of the JSON structure.
type dataFile struct {
	UnicodeVersion string    `json:"unicode_version"`
	GeneratedAt    time.Time `json:"generated_at"`
	SourceURL      string    `json:"source_url"`
	SourceDate     string    `json:"source_date"`
	Mappings       []Mapping `json:"mappings"`
}

// DB is the confusables database. Thread-safe after initialization.
type DB struct {
	mappings       map[rune][]rune
	unicodeVersion string
	sourceDate     string
	generatedAt    time.Time
	sourceURL      string
}

var (
	defaultDB *DB
	once      sync.Once
)

// Default returns the embedded database (loaded once via sync.Once).
func Default() *DB {
	once.Do(func() {
		db, err := Load(embeddedJSON)
		if err != nil {
			// This should never happen if the build process is correct
			panic("confusables: failed to load embedded data: " + err.Error())
		}
		defaultDB = db
	})
	return defaultDB
}

// Load creates a DB from a JSON file (for custom/updated data).
func Load(jsonData []byte) (*DB, error) {
	var df dataFile
	if err := json.Unmarshal(jsonData, &df); err != nil {
		return nil, fmt.Errorf("failed to unmarshal confusables data: %w", err)
	}

	db := &DB{
		mappings:       make(map[rune][]rune, len(df.Mappings)),
		unicodeVersion: df.UnicodeVersion,
		sourceDate:     df.SourceDate,
		generatedAt:    df.GeneratedAt,
		sourceURL:      df.SourceURL,
	}

	for _, m := range df.Mappings {
		if len(m.Target) == 0 {
			return nil, fmt.Errorf("invalid mapping for rune %04X: empty target", m.Source)
		}
		// Validate raw int before conversion to rune to avoid wrap-around truncation
		if m.Source < 0 || m.Source > 0x10FFFF || (m.Source >= 0xD800 && m.Source <= 0xDFFF) {
			return nil, fmt.Errorf("invalid unicode source codepoint: %04X", m.Source)
		}
		source := rune(m.Source)
		if _, exists := db.mappings[source]; exists {
			return nil, fmt.Errorf("duplicate mapping for rune %04X", m.Source)
		}
		// defensive copy and conversion to rune
		targets := make([]rune, len(m.Target))
		for i, t := range m.Target {
			if t < 0 || t > 0x10FFFF || (t >= 0xD800 && t <= 0xDFFF) {
				return nil, fmt.Errorf("invalid unicode target codepoint: %04X", t)
			}
			targets[i] = rune(t)
		}
		db.mappings[source] = targets
	}

	return db, nil
}

// UnicodeVersion returns the Unicode version of the database.
func (db *DB) UnicodeVersion() string {
	return db.unicodeVersion
}

// SourceDate returns the date of the source data.
func (db *DB) SourceDate() string {
	return db.sourceDate
}

// GeneratedAt returns the timestamp when the database was generated.
func (db *DB) GeneratedAt() time.Time {
	return db.generatedAt
}

// SourceURL returns the URL of the source data.
func (db *DB) SourceURL() string {
	return db.sourceURL
}

// ToASCII replaces confusable characters with their ASCII equivalents.
// Only replaces characters that map to a SINGLE ASCII char (0x20-0x7E).
// Characters with multi-char targets or non-ASCII targets are kept as-is.
// Already-ASCII characters (0x00-0x7F) are always returned unchanged.
func (db *DB) ToASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if replacement, ok := db.LookupASCII(r); ok {
			b.WriteRune(replacement)
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// Skeleton returns the TR39 skeleton of the string.
// Maps all confusable characters through the database, regardless of target length.
// Result is NOT suitable for display — use only for comparison.
// Implementation: NFD → map → NFD
func (db *DB) Skeleton(s string) string {
	// 1. NFD
	s = norm.NFD.String(s)

	// 2. Map
	var b strings.Builder
	for _, r := range s {
		if targets, ok := db.mappings[r]; ok {
			for _, tr := range targets {
				b.WriteRune(tr)
			}
		} else {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// 3. NFD again
	return norm.NFD.String(s)
}

// IsConfusable checks if two strings would produce the same skeleton.
func (db *DB) IsConfusable(a, b string) bool {
	return db.Skeleton(a) == db.Skeleton(b)
}

// LookupASCII returns the ASCII equivalent of a rune, if one exists.
// Returns (replacement, true) for single-char ASCII mappings.
// Returns (0, false) for characters with no mapping or multi-char/non-ASCII targets.
// Note: If the input rune is already ASCII (r < 0x80), it returns (0, false)
// to prevent remapping characters that are already valid ASCII.
func (db *DB) LookupASCII(r rune) (rune, bool) {
	if r < 0x80 {
		return 0, false
	}

	targets, ok := db.mappings[r]
	if !ok {
		return 0, false
	}

	if len(targets) == 1 && targets[0] >= 0x20 && targets[0] <= 0x7E {
		return targets[0], true
	}

	return 0, false
}

// Lookup returns all target runes for a confusable character.
// Returns a defensive copy of the internal slice.
// Returns nil if the character has no mapping.
func (db *DB) Lookup(r rune) []rune {
	targets, ok := db.mappings[r]
	if !ok {
		return nil
	}
	res := make([]rune, len(targets))
	copy(res, targets)
	return res
}
