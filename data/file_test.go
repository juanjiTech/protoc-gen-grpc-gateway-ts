package data

import (
	"path/filepath"
	"testing"
)

func TestFile(t *testing.T) {
	t.Log(filepath.Dir("data/file_test.go"))
	t.Log(filepath.Dir("data\\file_test.go"))
}
