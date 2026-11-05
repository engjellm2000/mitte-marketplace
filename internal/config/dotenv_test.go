package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDevelopmentEnvFileLoadsMissingProcessVariables(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FROM_PROCESS", "shell")
	file := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(file, []byte("# local settings\nFROM_FILE=loaded\nFROM_PROCESS=file\nQUOTED=\"hello world\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDevelopmentEnvFile(file); err != nil {
		t.Fatalf("LoadDevelopmentEnvFile() error = %v", err)
	}
	if got := os.Getenv("FROM_FILE"); got != "loaded" {
		t.Errorf("FROM_FILE = %q, want loaded", got)
	}
	if got := os.Getenv("FROM_PROCESS"); got != "shell" {
		t.Errorf("FROM_PROCESS = %q, want shell", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Errorf("QUOTED = %q, want hello world", got)
	}
}

func TestLoadDevelopmentEnvFileSkipsProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	file := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(file, []byte("ONLY_LOCAL=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDevelopmentEnvFile(file); err != nil {
		t.Fatalf("LoadDevelopmentEnvFile() error = %v", err)
	}
	if _, exists := os.LookupEnv("ONLY_LOCAL"); exists {
		t.Error("ONLY_LOCAL was set in production")
	}
}
