package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseConfusables(t *testing.T) {
	t.Run("Valid Input", func(t *testing.T) {
		input := `# Version: 16.0.0
# Date: 2024-08-14, 23:05:00 GMT
0430 ;	0061 ;	MA	# ( а → a ) CYRILLIC SMALL LETTER A → LATIN SMALL LETTER A
00DF ;	0073 0073 ; MA	# ( ß → ss ) LATIN SMALL LETTER SHARP S → LATIN SMALL LETTER S
`
		dataFile, err := parseConfusables(strings.NewReader(input), "test-url", "latest")
		if err != nil {
			t.Fatalf("parseConfusables failed: %v", err)
		}

		if dataFile.UnicodeVersion != "16.0.0" {
			t.Errorf("got version %q, want %q", dataFile.UnicodeVersion, "16.0.0")
		}
		if dataFile.SourceDate != "2024-08-14, 23:05:00 GMT" {
			t.Errorf("got date %q, want %q", dataFile.SourceDate, "2024-08-14, 23:05:00 GMT")
		}
		if len(dataFile.Mappings) != 2 {
			t.Errorf("got %d mappings, want 2", len(dataFile.Mappings))
		}
	})

	t.Run("Errors", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantErr string
		}{
			{
				name:    "Duplicate source",
				input:   "0430 ; 0061 ; MA # comment\n0430 ; 0062 ; MA # comment",
				wantErr: "duplicate source",
			},
			{
				name:    "Empty target",
				input:   "0430 ; ; MA # comment",
				wantErr: "empty target",
			},
			{
				name:    "Malformed missing comment",
				input:   "0430 ; 0061 ; MA",
				wantErr: "malformed line (missing comment)",
			},
			{
				name:    "Invalid source Unicode",
				input:   "D800 ; 0061 ; MA # comment",
				wantErr: "invalid unicode source",
			},
			{
				name:    "Invalid target Unicode",
				input:   "0430 ; D800 ; MA # comment",
				wantErr: "invalid unicode target",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := parseConfusables(strings.NewReader(tt.input), "test", "latest")
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			})
		}
	})

	t.Run("CLI Run", func(t *testing.T) {
		tmpInput := "test_confusables.txt"
		tmpOutput := "test_confusables.json"
		defer func() { _ = os.Remove(tmpInput) }()
		defer func() { _ = os.Remove(tmpOutput) }()

		content := "0430 ; 0061 ; MA # ( а → a ) CYRILLIC SMALL LETTER A → LATIN SMALL LETTER A\n"
		if err := os.WriteFile(tmpInput, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test input: %v", err)
		}

		err := run([]string{"--input", tmpInput, "--output", tmpOutput, "--version", "16.0.0"})
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}

		if _, err := os.Stat(tmpOutput); os.IsNotExist(err) {
			t.Fatal("output file was not created")
		}
	})
}
