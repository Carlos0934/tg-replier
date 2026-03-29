package internal_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// TestPackageLayoutReflectsUseCases is a structural test that verifies the
// screaming-architecture requirement: reading `internal/` directory names
// MUST reveal all active use-cases without reading any code.
//
// Spec scenario: "Package visibility from folder"
func TestPackageLayoutReflectsUseCases(t *testing.T) {
	// Resolve the internal/ directory relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path via runtime.Caller")
	}
	internalDir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(internalDir)
	if err != nil {
		t.Fatalf("reading internal/ directory: %v", err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)

	// Expected top-level packages per the spec. Each name must be a
	// recognisable use-case or infrastructure adapter — no generic layers
	// like "handlers", "models", or "utils".
	expected := []string{
		"commands", // slash-command routing use-case
		"config",   // app configuration
		"groups",   // group management use-case
		"members",  // known-member tracking per chat
		"storage",  // persistence adapters
		"telegram", // transport adapter
	}

	if len(dirs) != len(expected) {
		t.Fatalf("expected %d packages %v, got %d: %v", len(expected), expected, len(dirs), dirs)
	}
	for i, name := range expected {
		if dirs[i] != name {
			t.Errorf("internal/[%d]: expected %q, got %q", i, name, dirs[i])
		}
	}

	// Additionally verify that no banned layer-oriented names exist.
	banned := map[string]bool{
		"handlers":    true,
		"models":      true,
		"utils":       true,
		"helpers":     true,
		"controllers": true,
		"services":    true,
		"middleware":  true,
		"bot":         true, // old package name
	}
	for _, d := range dirs {
		if banned[d] {
			t.Errorf("internal/ contains banned layer-oriented package %q", d)
		}
	}
}
