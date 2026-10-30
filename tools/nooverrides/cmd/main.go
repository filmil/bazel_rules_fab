// SPDX-License-Identifier: Apache-2.0

// Command nooverrides fails when a module overrides a dependency:
//
//	bazel run //tools/nooverrides -- "$PWD"
//	bazel run //tools/nooverrides -- --allow_local_path=hdlf_rules_fab "$PWD/integration"
//
// Each argument is a directory with a MODULE.bazel, or the file itself.
// --allow_local_path=NAME allows, and requires, one local_path_override of
// NAME: the integration module's of this module.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/filmil/bazel_rules_fab/tools/nooverrides"
)

func main() {
	allow := flag.String("allow_local_path", "", "a module whose one local_path_override is allowed and required")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "nooverrides: give a module directory or MODULE.bazel")
		os.Exit(2)
	}
	bad := false
	for _, p := range flag.Args() {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			p = filepath.Join(p, "MODULE.bazel")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "nooverrides:", err)
			os.Exit(2)
		}
		for _, problem := range nooverrides.Check(p, string(b), *allow) {
			fmt.Println(problem)
			bad = true
		}
	}
	if bad {
		os.Exit(1)
	}
	fmt.Println("no overrides")
}
