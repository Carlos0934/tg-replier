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

// --- Spec: Multi-Stage Dockerfile ---

// Scenario: Successful image build (structural validation)
// Verifies the Dockerfile exists and uses a multi-stage build with required
// properties: static compilation, minimal runtime, CA certs, non-root user.
func TestDockerfile_MultiStageBuild(t *testing.T) {
	content := rootFile(t, "Dockerfile")

	// Must have at least two FROM instructions (multi-stage)
	fromCount := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			fromCount++
		}
	}
	if fromCount < 2 {
		t.Errorf("Dockerfile must be multi-stage (expected >= 2 FROM instructions, got %d)", fromCount)
	}

	// Must compile with CGO_ENABLED=0 for a static binary
	if !strings.Contains(content, "CGO_ENABLED=0") {
		t.Error("Dockerfile must compile with CGO_ENABLED=0 for a static binary")
	}

	// Must install CA certificates for TLS to api.telegram.org
	if !strings.Contains(content, "ca-certificates") {
		t.Error("Dockerfile must install ca-certificates for outbound TLS")
	}
}

// Scenario: Non-root execution
// The Dockerfile must declare a non-root USER.
func TestDockerfile_NonRootUser(t *testing.T) {
	content := rootFile(t, "Dockerfile")

	scanner := bufio.NewScanner(strings.NewReader(content))
	hasUser := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToUpper(line), "USER ") {
			user := strings.TrimSpace(line[5:])
			if user != "root" && user != "" {
				hasUser = true
			}
		}
	}
	if !hasUser {
		t.Error("Dockerfile must declare a non-root USER")
	}

	// Must NOT have EXPOSE (worker-only bot, outbound polling)
	if strings.Contains(strings.ToUpper(content), "EXPOSE ") {
		t.Error("Dockerfile must NOT expose any ports (bot uses outbound polling only)")
	}
}

// --- Spec: Build Context Exclusion (.dockerignore) ---

// Scenario: Clean build context
// .dockerignore must exclude non-essential files per the spec.
func TestDockerignore_ExcludesRequiredPatterns(t *testing.T) {
	content := rootFile(t, ".dockerignore")

	// Spec-required exclusions
	required := []string{
		".git/",
		"data/",
		".env",
		"*.md",
		".agents/",
		".atl/",
		"*.test",
	}
	for _, pattern := range required {
		if !strings.Contains(content, pattern) {
			t.Errorf(".dockerignore must exclude %q", pattern)
		}
	}
}

// --- Spec: Runtime Environment Variables ---

// Scenario: Dockerfile sets DATA_DIR default
// The Dockerfile must set ENV DATA_DIR to a predictable container path.
func TestDockerfile_DataDirDefault(t *testing.T) {
	content := rootFile(t, "Dockerfile")

	if !strings.Contains(content, "ENV DATA_DIR=") {
		t.Error("Dockerfile must set ENV DATA_DIR to a default container path")
	}
}

// --- Spec: Deployment Documentation (README) ---

// Scenario: README documents Docker deployment
// README must contain Docker build, run, and Dokploy instructions.
func TestReadme_DockerDeploymentSection(t *testing.T) {
	content := rootFile(t, "README.md")

	checks := []struct {
		search string
		label  string
	}{
		{"## Docker", "Docker section heading"},
		{"docker build", "docker build command"},
		{"docker run", "docker run command"},
		{"BOT_TOKEN", "BOT_TOKEN environment variable"},
		{"/app/data", "data volume mount path"},
		{"Dokploy", "Dokploy deployment reference"},
		{"volume", "volume persistence guidance"},
		{"non-root", "non-root user documentation"},
	}
	for _, c := range checks {
		if !strings.Contains(content, c.search) {
			t.Errorf("README Docker section must contain %s (%q)", c.label, c.search)
		}
	}
}
