// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "gerbers")
	if err := os.Mkdir(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "sub", "out.pdf")
	if err := Write(file+"\n"+tree+"\n", []string{"sch", "export", "pdf"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{file, filepath.Join(tree, "stub_record")} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), Marker+"\n") || !strings.Contains(string(b), `["sch","export","pdf"]`) {
			t.Errorf("%s: got %q", p, b)
		}
	}
}
