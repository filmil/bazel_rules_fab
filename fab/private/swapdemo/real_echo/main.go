// SPDX-License-Identifier: Apache-2.0

// Command real_echo is the "real" tool of the swap test template: it
// writes "real" to the file named by its first argument.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: real_echo OUT")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], []byte("real\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
