// SPDX-License-Identifier: Apache-2.0

// Command detcompare is the test program of determinism_test.
//
// It compares two builds of the same outputs, given as two lists of
// runfiles paths in DET_A and DET_B (same order), by the SHA-256 of every
// regular file, directories walked. With DET_EXPECT=same (the default) it
// fails on any difference; with DET_EXPECT=different it fails when there
// is none, which is how determinism_test checks itself on a fixture that
// writes the time.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Hashes returns the SHA-256 of each regular file under root, keyed by its
// path relative to root ("." for root itself when it is a file). root may
// be a symlink, as outputs in a test's runfiles are, also to a directory.
func Hashes(root string) (map[string]string, error) {
	out := map[string]string{}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return out, err
}

// Diff compares the outputs a[i] and b[i] and returns one line per file
// that differs or exists on one side only.
func Diff(a, b []string) ([]string, error) {
	if len(a) != len(b) {
		return nil, fmt.Errorf("the two builds have %d and %d outputs", len(a), len(b))
	}
	var diffs []string
	for i := range a {
		ha, err := Hashes(a[i])
		if err != nil {
			return nil, err
		}
		hb, err := Hashes(b[i])
		if err != nil {
			return nil, err
		}
		keys := map[string]bool{}
		for k := range ha {
			keys[k] = true
		}
		for k := range hb {
			keys[k] = true
		}
		var sorted []string
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			name := filepath.ToSlash(filepath.Join(filepath.Base(a[i]), k))
			switch {
			case ha[k] == "":
				diffs = append(diffs, name+": only in the second build")
			case hb[k] == "":
				diffs = append(diffs, name+": only in the first build")
			case ha[k] != hb[k]:
				diffs = append(diffs, fmt.Sprintf("%s: sha256 %s vs %s", name, ha[k][:12], hb[k][:12]))
			}
		}
	}
	return diffs, nil
}

// Lines splits a list of paths, one per line, so a path may hold spaces.
func Lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func main() {
	a, b := Lines(os.Getenv("DET_A")), Lines(os.Getenv("DET_B"))
	if len(a) == 0 {
		fmt.Fprintln(os.Stderr, "detcompare: no outputs to compare")
		os.Exit(1)
	}
	diffs, err := Diff(a, b)
	if err != nil {
		fmt.Fprintln(os.Stderr, "detcompare:", err)
		os.Exit(2)
	}
	if os.Getenv("DET_EXPECT") == "different" {
		if len(diffs) == 0 {
			fmt.Println("detcompare: expected the builds to differ, but they are identical")
			os.Exit(1)
		}
		fmt.Printf("detcompare: the builds differ, as expected (%d file(s))\n", len(diffs))
		return
	}
	for _, d := range diffs {
		fmt.Println("not deterministic:", d)
	}
	if len(diffs) > 0 {
		os.Exit(1)
	}
	fmt.Printf("detcompare: %d output(s) identical in both builds\n", len(a))
}
