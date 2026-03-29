package main_test

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// rootFile reads a file from the project root (test working dir).
func rootFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	return string(b)
}

// --- Spec: README exists and is complete ---

// Scenario: Developer clones the repo and reads README
// Verifies that README.md exists and contains all required sections so a
// developer can run the project without consulting any other file.
func TestReadme_ContainsRequiredSections(t *testing.T) {
	content := rootFile(t, "README.md")

	requiredSections := []string{
		"## Prerequisites",
		"## Setup",
		"## Environment Variables",
		"## Bot Commands",
		"## Architecture",
	}
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("README.md missing required section: %s", section)
		}
	}

	// Must mention project purpose in the opening paragraph
	if !strings.Contains(content, "Telegram") {
		t.Error("README.md does not mention Telegram in project description")
	}
}

// Scenario: README references correct env vars
// Every variable in .env.example must appear in the README with a description,
// and no undocumented variable is required to start the app.
func TestReadme_EnvVarsMatchTemplate(t *testing.T) {
	readme := rootFile(t, "README.md")
	envExample := rootFile(t, ".env.example")

	// Extract variable names from .env.example (lines matching KEY=value, ignoring comments)
	var envVars []string
	scanner := bufio.NewScanner(strings.NewReader(envExample))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and pure comments
		if line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, "=")) {
			continue
		}
		// Handle commented-out optional vars like "# DATA_DIR=data"
		line = strings.TrimPrefix(line, "# ")
		if idx := strings.Index(line, "="); idx > 0 {
			varName := strings.TrimSpace(line[:idx])
			envVars = append(envVars, varName)
		}
	}

	if len(envVars) == 0 {
		t.Fatal(".env.example contains no environment variables")
	}

	for _, v := range envVars {
		if !strings.Contains(readme, v) {
			t.Errorf("README.md does not document env var %q from .env.example", v)
		}
	}
}

// --- Spec: .env.example template exists ---

// Scenario: Developer copies template to start
// .env.example must exist and contain a BOT_TOKEN placeholder.
func TestEnvExample_Exists_WithBotToken(t *testing.T) {
	content := rootFile(t, ".env.example")

	if !strings.Contains(content, "BOT_TOKEN=") {
		t.Error(".env.example does not contain BOT_TOKEN variable")
	}
}

// Scenario: Template contains no real secrets
// All values in .env.example must be placeholders — no actual tokens or credentials.
func TestEnvExample_NoRealSecrets(t *testing.T) {
	content := rootFile(t, ".env.example")

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(line[idx+1:])
		// Values should be empty or clearly placeholder text
		if value == "" {
			continue
		}
		// Telegram bot tokens match the pattern digits:alphanumeric
		if strings.Contains(value, ":") && len(value) > 20 {
			t.Errorf("possible real token in .env.example: %s=<redacted>", line[:idx])
		}
		// Placeholder values should contain words like "your", "here", "example", etc.
		lower := strings.ToLower(value)
		isPlaceholder := strings.Contains(lower, "your") ||
			strings.Contains(lower, "here") ||
			strings.Contains(lower, "example") ||
			strings.Contains(lower, "data") ||
			strings.Contains(lower, "placeholder")
		if !isPlaceholder {
			t.Errorf("value for %s does not look like a placeholder: %q", line[:idx], value)
		}
	}
}

// --- Spec: .env is excluded from version control ---

// Scenario: Developer creates .env locally
// .gitignore must contain a line that excludes .env files.
func TestGitignore_ExcludesEnv(t *testing.T) {
	content := rootFile(t, ".gitignore")

	scanner := bufio.NewScanner(strings.NewReader(content))
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match exact ".env" or patterns like ".env*"
		if line == ".env" || line == ".env*" || line == "*.env" {
			found = true
			break
		}
	}
	if !found {
		t.Error(".gitignore does not contain .env exclusion rule")
	}
}
