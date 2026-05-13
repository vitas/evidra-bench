package scenario

import (
	"path/filepath"
	"runtime"
)

type resourceRef struct {
	kind      string
	namespace string
	name      string
}

func runtimeProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}
