//nolint:testpackage // white-box tests for unexported helpers.
package configuration

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const configFileName = "config.yaml"

func TestCreateOrUpdateSeedsDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)

	if err := CreateOrUpdate(path); err != nil {
		t.Fatalf("CreateOrUpdate() error: %v", err)
	}

	cfg, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if cfg.Version != Version {
		t.Fatalf("Version = %q, want %q", cfg.Version, Version)
	}

	if !slices.Equal(cfg.Sources, sources.List) {
		t.Fatalf("Sources = %v, want %v", cfg.Sources, sources.List)
	}

	for _, name := range []string{
		sources.Bevigil,
		sources.Github,
		sources.Gitlab,
		sources.IntelligenceX,
		sources.Leakradar,
		sources.OpenThreatExchange,
		sources.URLScan,
		sources.Virustotal,
		sources.Zoomeye,
	} {
		if _, ok := cfg.Keys[name]; !ok {
			t.Fatalf("missing key slot %q", name)
		}
	}
}

func TestCreateOrUpdateRecoversEmptyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := CreateOrUpdate(path); err != nil {
		t.Fatalf("CreateOrUpdate() error: %v", err)
	}

	cfg, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if !slices.Equal(cfg.Sources, sources.List) {
		t.Fatalf("Sources = %v, want defaults", cfg.Sources)
	}
}

func TestCreateOrUpdatePreservesSelectionAndKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)
	content := "version: old\n" +
		"sources:\n" +
		"    - wayback\n" +
		"keys:\n" +
		"    bevigil:\n" +
		"        - secret\n"

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := CreateOrUpdate(path); err != nil {
		t.Fatalf("CreateOrUpdate() error: %v", err)
	}

	cfg, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if !slices.Equal(cfg.Sources, []string{sources.Wayback}) {
		t.Fatalf("Sources = %v, want [%s]", cfg.Sources, sources.Wayback)
	}

	if !slices.Equal(cfg.Keys[sources.Bevigil], sources.Keys{"secret"}) {
		t.Fatalf("bevigil keys = %v, want [secret]", cfg.Keys[sources.Bevigil])
	}

	if _, ok := cfg.Keys[sources.Github]; !ok {
		t.Fatal("missing github key slot after rewrite")
	}

	if cfg.Version != Version {
		t.Fatalf("Version = %q, want %q", cfg.Version, Version)
	}
}

func TestReadRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)

	if err := os.WriteFile(path, []byte("keys: [unclosed"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if _, err := Read(path); err == nil {
		t.Fatal("Read() = nil error, want parse error")
	}
}

func TestWriteIsAtomicAndPrivate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	cfg := DefaultConfiguration

	if err := cfg.Write(path); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != configFileName {
		t.Fatalf("directory entries = %v, want only %q", entries, configFileName)
	}

	if runtime.GOOS == "windows" {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o, want 600", mode)
	}
}

func TestWriteRemovesTemporaryFileOnRenameFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")

	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	cfg := DefaultConfiguration

	if err := cfg.Write(target); err == nil {
		t.Fatal("Write() = nil error, want rename failure")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "target" {
		t.Fatalf("directory entries = %v, want only %q", entries, "target")
	}
}

func TestHasMissingKeySlots(t *testing.T) {
	t.Parallel()

	complete := Configuration{
		Keys: make(map[string]sources.Keys, len(DefaultConfiguration.Keys)),
	}

	for name := range DefaultConfiguration.Keys {
		complete.Keys[name] = sources.Keys{}
	}

	if hasMissingKeySlots(&complete) {
		t.Fatal("hasMissingKeySlots() = true for complete configuration")
	}

	delete(complete.Keys, sources.Github)

	if !hasMissingKeySlots(&complete) {
		t.Fatal("hasMissingKeySlots() = false for configuration missing a slot")
	}
}
