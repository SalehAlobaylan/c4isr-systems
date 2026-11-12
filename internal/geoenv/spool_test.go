package geoenv

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanSpoolKeepsActiveAndUnmanagedDirectories(t *testing.T) {
	root, now := t.TempDir(), time.Now()
	for name, age := range map[string]time.Duration{
		"emad-environment-1":     36 * time.Minute,
		"emad-environment-2":     10 * time.Minute,
		"emad-environment-other": 36 * time.Minute,
		"unrelated":              36 * time.Minute,
	} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	if err := CleanSpool(root, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "emad-environment-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected abandoned directory to be removed, got %v", err)
	}
	for _, name := range []string{"emad-environment-2", "emad-environment-other", "unrelated"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("expected %s to remain: %v", name, err)
		}
	}
}
