package scenarios

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

var validScenarioName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Loader reads scenario definitions from disk. Scenario files are the only
// external input to the runner; nothing else is fetched.
type Loader struct {
	dir string
}

// NewLoader builds a loader for a scenarios directory.
func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

// Load parses and validates one scenario by name.
func (l *Loader) Load(name string) (*Scenario, error) {
	if !validScenarioName.MatchString(name) {
		return nil, apperr.NotFound("scenario", name)
	}

	var raw []byte
	var err error
	for _, ext := range []string{".yaml", ".yml"} {
		raw, err = os.ReadFile(filepath.Join(l.dir, name+ext))
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if raw == nil {
		return nil, apperr.NotFound("scenario", name)
	}

	var sc Scenario
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return nil, apperr.Validation("invalid scenario YAML: " + err.Error())
	}
	if err := sc.Validate(); err != nil {
		return nil, err
	}
	return &sc, nil
}

// List returns summaries of every valid scenario file in the directory.
func (l *Loader) List() ([]Summary, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, err
	}

	summaries := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		name = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")

		sc, err := l.Load(name)
		if err != nil {
			continue
		}
		summaries = append(summaries, sc.Summary())
	}
	return summaries, nil
}
