// SPDX-License-Identifier: Apache-2.0

// Command swapcheck is the test program of swap_test.
//
// It takes the outputs of the targets under test, built with every tool
// swapped for the stub, and checks that each regular file (inside
// directories too) starts with the stub's record. A file that does not was
// written by something other than a toolchain tool: a hidden host tool, or
// an action outside run_tool.
package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Marker must start every checked file. It matches the stub's Marker.
const Marker = "RUN_TOOL_STUB"

// Check returns the files under paths that do not start with Marker, and
// how many files it checked.
func Check(paths []string) (bad []string, n int, err error) {
	for _, root := range paths {
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			n++
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			line, _ := bufio.NewReader(f).ReadString('\n')
			if line != Marker+"\n" {
				bad = append(bad, p)
			}
			return nil
		})
		if err != nil {
			return nil, n, err
		}
	}
	return bad, n, nil
}

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		// swap_test passes the outputs here, relative to the runfiles root.
		paths = strings.Fields(os.Getenv("SWAP_FILES"))
	}
	bad, n, err := Check(paths)
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
	if len(bad) > 0 {
		os.Exit(1)
	}
	fmt.Printf("swapcheck: all %d output files came from the stub\n", n)
}
