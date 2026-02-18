package confusables

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestDBBasics(t *testing.T) {
	db := Default()

	t.Run("ToASCII", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"hello", "hello"},
			{"hеllо", "hello"},   // Cyrillic 'е' and 'о'
			{"vіаgrа", "viagra"}, // Cyrillic 'а' and Latin 'і'
			{"123", "123"},       // Digit '1' is already ASCII
			{"paypаl", "paypal"}, // Cyrillic 'а'
			{"ß", "ß"},           // Multi-char mapping
		}

		for _, tt := range tests {
			if got := db.ToASCII(tt.input); got != tt.expected {
				t.Errorf("ToASCII(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		}
	})

	t.Run("Skeleton and IsConfusable", func(t *testing.T) {
		tests := []struct {
			a    string
			b    string
			conf bool
		}{
			{"hello", "hello", true},
			{"hello", "hеllо", true},
			{"viagra", "vіаgrа", true},
			{"paypal", "pаypаl", true},
			{"apple", "аррle", true},
			{"123", "l23", true},
			{"different", "strings", false},
		}

		for _, tt := range tests {
			if got := db.IsConfusable(tt.a, tt.b); got != tt.conf {
				t.Errorf("IsConfusable(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.conf)
			}
		}
	})

	t.Run("Lookup", func(t *testing.T) {
		tests := []struct {
			r        rune
			expected []rune
		}{
			{'а', []rune{'a'}},
			{8353, []rune{67, 8427}},
			{'z', nil},
		}

		for _, tt := range tests {
			got := db.Lookup(tt.r)
			if len(got) != len(tt.expected) {
				t.Errorf("Lookup(%c) got len %d, want %d", tt.r, len(got), len(tt.expected))
				continue
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Lookup(%c)[%d] = %c, want %c", tt.r, i, got[i], tt.expected[i])
				}
			}
		}
	})
}

func TestImmutability(t *testing.T) {
	db := Default()
	r := 'а'
	targets := db.Lookup(r)
	if len(targets) == 0 {
		t.Fatal("Lookup('а') returned nil")
	}

	targets[0] = 'Z'
	if db.Lookup(r)[0] == 'Z' {
		t.Error("Internal state mutated!")
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    dataFile
		wantErr string
	}{
		{
			name: "Empty target",
			data: dataFile{
				Mappings: []Mapping{
					{Source: 0x41, Target: []int{}},
				},
			},
			wantErr: "empty target",
		},
		{
			name: "Duplicate source",
			data: dataFile{
				Mappings: []Mapping{
					{Source: 0x41, Target: []int{0x41}},
					{Source: 0x41, Target: []int{0x42}},
				},
			},
			wantErr: "duplicate mapping",
		},
		{
			name: "Invalid source Unicode",
			data: dataFile{
				Mappings: []Mapping{
					{Source: 0xD800, Target: []int{0x41}},
				},
			},
			wantErr: "invalid unicode source",
		},
		{
			name: "Invalid target Unicode",
			data: dataFile{
				Mappings: []Mapping{
					{Source: 0x41, Target: []int{0xD800}},
				},
			},
			wantErr: "invalid unicode target",
		},
		{
			name: "Oversized target Unicode (wrap-around)",
			data: dataFile{
				Mappings: []Mapping{
					{Source: 0x41, Target: []int{4294967361}}, // 0x100000041, wraps to 0x41 ('A') if cast directly
				},
			},
			wantErr: "invalid unicode target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.data)
			_, err := Load(jsonData)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %v, wantErr %q", err, tt.wantErr)
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	db := Default()
	if db.UnicodeVersion() == "" {
		t.Error("UnicodeVersion() is empty")
	}
	if db.SourceDate() == "" {
		t.Error("SourceDate() is empty")
	}
	if db.SourceURL() == "" {
		t.Error("SourceURL() is empty")
	}
	if db.GeneratedAt().IsZero() {
		t.Error("GeneratedAt() is zero")
	}
}

func TestConcurrency(_ *testing.T) {
	db := Default()
	const (
		goroutines = 100
		iterations = 1000
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = db.ToASCII("vіаgrа")
				_ = db.Skeleton("аррle")
				_ = db.IsConfusable("paypal", "pаypаl")
				_ = db.Lookup('а')
			}
		}()
	}
	wg.Wait()
}
