package scenarios

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
)

var buildOnce sync.Once
var buildDigest string

// Hash the running executable once, including dirty local builds. A release
// label alone does not identify the code that generated a run.
func runtimeBuild() string {
	buildOnce.Do(func() {
		path, err := os.Executable()
		if err != nil {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		h := sha256.New()
		if _, err = io.Copy(h, f); err == nil {
			buildDigest = fmt.Sprintf("sha256:%x", h.Sum(nil))
		}
	})
	return buildDigest
}
