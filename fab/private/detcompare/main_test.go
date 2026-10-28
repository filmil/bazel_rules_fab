// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	d := t.TempDir()
	a, b := filepath.Join(d, "a"), filepath.Join(d, "b")
	write(t, filepath.Join(a, "board.pdf"), "same")
	write(t, filepath.Join(b, "board.pdf"), "same")
	write(t, filepath.Join(a, "gerbers", "F_Cu.gbr"), "%TF.CreationDate,2026-10-28T05:00:00*%")
	write(t, filepath.Join(b, "gerbers", "F_Cu.gbr"), "%TF.CreationDate,2026-10-28T05:00:01*%")
	write(t, filepath.Join(a, "gerbers", "B_Cu.gbr"), "x")
	write(t, filepath.Join(b, "gerbers", "Edge.gbr"), "y")

	got, err := Diff(
		[]string{filepath.Join(a, "board.pdf"), filepath.Join(a, "gerbers")},
		[]string{filepath.Join(b, "board.pdf"), filepath.Join(b, "gerbers")},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gerbers/B_Cu.gbr: only in the first build",
		"gerbers/Edge.gbr: only in the second build",
		"gerbers/F_Cu.gbr: sha256 ",
	}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2][:len(want[2])] != want[2] {
		t.Errorf("got %q", got)
	}

	// Outputs in runfiles are symlinks, a directory output too.
	link := filepath.Join(d, "link")
	if err := os.Symlink(filepath.Join(a, "gerbers"), link); err != nil {
		t.Fatal(err)
	}
	viaLink, err := Diff([]string{link}, []string{filepath.Join(b, "gerbers")})
	if err != nil || len(viaLink) != 3 {
		t.Errorf("through a symlink: got %q, %v", viaLink, err)
	}
	if got := Lines("a b/c.pdf\n\nd\n"); !reflect.DeepEqual(got, []string{"a b/c.pdf", "d"}) {
		t.Errorf("Lines: got %q", got)
	}

	same, err := Diff([]string{filepath.Join(a, "board.pdf")}, []string{filepath.Join(b, "board.pdf")})
	if err != nil || !reflect.DeepEqual(same, []string(nil)) {
		t.Errorf("identical files: got %q, %v", same, err)
	}
	if _, err := Diff([]string{a}, nil); err == nil {
		t.Error("different counts: want an error")
	}
}
