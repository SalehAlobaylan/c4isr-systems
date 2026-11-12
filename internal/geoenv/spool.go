package geoenv

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CleanSpool removes directories left by a process death after an entire
// attempt lifetime and shutdown grace have passed. The root is private to
// this installation. Directory symlinks and unknown names are ignored.
func CleanSpool(root string, now time.Time) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	cutoff := now.Add(-attemptTimeout - 5*time.Minute)
	for _, entry := range entries {
		suffix, matches := strings.CutPrefix(entry.Name(), attemptDirectoryPrefix)
		if !matches || !entry.IsDir() {
			continue
		}
		if _, err := strconv.ParseUint(suffix, 10, 32); err != nil {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue // Another worker already removed the directory.
		}
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
