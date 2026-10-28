// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) string {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("a b.pdf", Marker+"\n[\"x\"]\n")
	write("tree/stub_record", Marker+"\n[]\n")
	write("tree/F_Cu.gbr", "%TF.GenerationSoftware,KiCad*%\n")
	near := write("b.txt", Marker+"X\n")
	empty := write("empty.txt", "")

	// Outputs in runfiles are symlinks, a directory output too.
	links := filepath.Join(dir, "runfiles")
	if err := os.Mkdir(links, 0o755); err != nil {
		t.Fatal(err)
	}
	treeLink := filepath.Join(links, "tree")
	if err := os.Symlink(filepath.Join(dir, "tree"), treeLink); err != nil {
		t.Fatal(err)
	}
	goodLink := filepath.Join(links, "a b.pdf")
	if err := os.Symlink(good, goodLink); err != nil {
		t.Fatal(err)
	}

	bad, n, err := Check([]string{goodLink, treeLink, near, empty})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(treeLink, "F_Cu.gbr"), near, empty}
	if n != 5 || !reflect.DeepEqual(bad, want) {
		t.Errorf("got bad=%v n=%d, want %v and 5", bad, n, want)
	}

	if _, _, err := Check([]string{filepath.Join(dir, "missing")}); err == nil {
		t.Error("a missing output: want an error")
	}
}

func TestPaths(t *testing.T) {
	if got := Paths(nil, "a b/c.pdf\n\nd\n"); !reflect.DeepEqual(got, []string{"a b/c.pdf", "d"}) {
		t.Errorf("got %q", got)
	}
	if got := Paths([]string{"x"}, "y"); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("got %q", got)
	}
}
