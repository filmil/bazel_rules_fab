// SPDX-License-Identifier: Apache-2.0

// Command clock is a deliberately non-deterministic tool: it writes the
// current time, in nanoseconds, to the file named by its first argument.
// determinism_test's own test uses it as its known-bad input (spec T3).
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: clock OUT")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], []byte(fmt.Sprintf("%d\n", time.Now().UnixNano())), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
