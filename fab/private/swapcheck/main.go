// SPDX-License-Identifier: Apache-2.0

// Command swapcheck is the test program of swap_test.
//
// With SWAP_EXPECT=foreign it inverts: it fails unless some file lacks the
// record. swap_test's own test uses that on a known-bad target.
//
// It takes the outputs of the targets under test, built with every tool
// swapped for the stub, and checks that each regular file (inside
// directories too) starts with the stub's record. A file that does not was
// written by something other than a toolchain tool: a hidden host tool, or
// an action outside run_tool.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Marker must start every checked file. It matches the stub's Marker.
const Marker = "RUN_TOOL_STUB"

// Check returns the files under paths that do not start with Marker, and
// how many files it checked. A path may be a symlink, as outputs in a
// test's runfiles are, also to a directory.
func Check(paths []string) (bad []string, n int, err error) {
	for _, p := range paths {
		root, err := filepath.EvalSymlinks(p)
		if err != nil {
			return nil, n, err
		}
		err = filepath.WalkDir(root, func(f string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			n++
			ok, err := startsWithMarker(f)
			if err != nil {
				return err
			}
			if !ok {
				rel, _ := filepath.Rel(root, f)
				bad = append(bad, filepath.Join(p, rel))
			}
			return nil
		})
		if err != nil {
			return nil, n, err
		}
	}
	return bad, n, nil
}

func startsWithMarker(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return line == Marker+"\n", nil
}

// Paths returns the outputs to check: the arguments, or else the lines of
// SWAP_FILES, which swap_test sets (one runfiles path per line, so a path
// may hold spaces).
func Paths(args []string, env string) []string {
	if len(args) > 0 {
		return args
	}
	var out []string
	for _, l := range strings.Split(env, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func main() {
	bad, n, err := Check(Paths(os.Args[1:], os.Getenv("SWAP_FILES")))
	if err != nil {
		fmt.Fprintln(os.Stderr, "swapcheck:", err)
		os.Exit(2)
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "swapcheck: no outputs to check")
		os.Exit(1)
	}
	for _, b := range bad {
		fmt.Printf("not made by a toolchain tool: %s\n", b)
	}
	if os.Getenv("SWAP_EXPECT") == "foreign" {
		// swap_test's own known-bad test: the check must find something.
		if len(bad) == 0 {
			fmt.Println("swapcheck: expected an output not made by the stub, found none")
			os.Exit(1)
		}
		fmt.Println("swapcheck: found the output not made by the stub, as expected")
		return
	}
	if len(bad) > 0 {
		os.Exit(1)
	}
	fmt.Printf("swapcheck: all %d output files came from the stub\n", n)
}
