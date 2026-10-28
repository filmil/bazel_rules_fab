// SPDX-License-Identifier: Apache-2.0

// Command real_echo is the "real" tool of the swap test template: it
// writes "real" to the file named by its first argument, or to out.txt
// inside it when that is a directory (a tree artifact Bazel made).
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: real_echo OUT")
		os.Exit(2)
	}
	out := os.Args[1]
	if fi, err := os.Stat(out); err == nil && fi.IsDir() {
		out = filepath.Join(out, "out.txt")
	}
	if err := os.WriteFile(out, []byte("real\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
