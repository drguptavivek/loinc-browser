package loinc

import (
	"database/sql"
	"strings"
	"testing"
)

// TestRegexpSQLFunctionAnchored guards the $expand `regex` compose filter's real-regex behavior
// (item 5 of the FHIR terminology review): "^Gluc" must anchor to the start of the value, not
// behave like a LIKE "%Gluc%" substring match.
func TestRegexpSQLFunctionAnchored(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer db.Close()

	cases := []struct {
		pattern, value string
		want           bool
	}{
		{"^Gluc", "Glucose", true},
		{"^Gluc", "Blood Glucose", false},
		{"^Chol", "Cholesterol", true},
		{"^hol", "Cholesterol", false},
	}
	for _, tc := range cases {
		var matched int
		if err := db.QueryRow(`select regexp(?, ?)`, tc.pattern, tc.value).Scan(&matched); err != nil {
			t.Fatalf("regexp(%q, %q): %v", tc.pattern, tc.value, err)
		}
		if got := matched == 1; got != tc.want {
			t.Errorf("regexp(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
		}
	}
}

func TestValidateRegexFilter(t *testing.T) {
	if err := ValidateRegexFilter("^Gluc"); err != nil {
		t.Fatalf("expected a valid pattern to pass, got %v", err)
	}
	if err := ValidateRegexFilter(strings.Repeat("a", 257)); err == nil {
		t.Fatal("expected an oversized pattern to be rejected")
	}
	if err := ValidateRegexFilter("(unclosed"); err == nil {
		t.Fatal("expected an uncompilable pattern to be rejected")
	}
}
