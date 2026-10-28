// SPDX-License-Identifier: Apache-2.0

// Command stub stands in for any tool in the swap test.
//
// It creates every output the launcher lists in RUN_TOOL_OUTPUTS and writes
// into each a record: the Marker line, then its own arguments as JSON. An
// output that Bazel made as a directory (a tree artifact) gets the record in
// a file named stub_record inside it. The swap test then checks that every
// output of the targets under test holds a record, which shows that every
// action ran a tool from a toolchain.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marker starts every record.
const Marker = "RUN_TOOL_STUB"

// Record returns what the stub writes for args.
func Record(args []string) []byte {
	j, _ := json.Marshal(args)
	return []byte(Marker + "\n" + string(j) + "\n")
}

// Write creates each output in outputs (one path per line) with the record.
func Write(outputs string, args []string) error {
	rec := Record(args)
	for _, p := range strings.Split(outputs, "\n") {
		if p == "" {
			continue
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			p = filepath.Join(p, "stub_record")
		} else if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, rec, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := Write(os.Getenv("RUN_TOOL_OUTPUTS"), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "stub:", err)
		os.Exit(1)
	}
}
