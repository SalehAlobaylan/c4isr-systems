package geoenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Opt in to validate the real offline processor's contract with the Go
// publisher without granting the test any source or release authority.
func TestPreparedCandidatePublisherContract(t *testing.T) {
	root := os.Getenv("C4ISR_TEST_ENVIRONMENT_CANDIDATES")
	if root == "" {
		t.Skip("supply the retained real candidate directory")
	}
	for _, area := range []string{"jeddah", "al-ain", "avignon"} {
		t.Run(area, func(t *testing.T) {
			dir := filepath.Join(root, area+"-candidate")
			raw, err := readSmall(filepath.Join(dir, "recipe.json"), 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			var prepared struct {
				Recipe
				InputSHA256 string `json:"inputSha256"`
			}
			if err = json.Unmarshal(raw, &prepared); err != nil {
				t.Fatal(err)
			}
			p, ok := profile(prepared.Recipe.Profile.ID)
			if !ok {
				t.Fatal("unknown profile")
			}
			prepared.Recipe.Profile = p
			raw, err = readSmall(filepath.Join(dir, "manifest.json"), 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			m, err := validateManifest(raw, Job{recipe: prepared.Recipe, InputSHA256: prepared.InputSHA256})
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(filepath.Join(dir, "prepared.zip"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			stat, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err = zipIntegrity(f, stat.Size(), raw, m); err != nil {
				t.Fatal(err)
			}
		})
	}
}
