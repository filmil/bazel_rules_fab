// SPDX-License-Identifier: Apache-2.0

// Package nooverrides finds the dependency overrides in a MODULE.bazel.
//
// An override in a root module bypasses the registry, so
// `bazel mod graph --registry=https://bcr.bazel.build` passes with one, but
// the BCR rejects a module that has one (issue #7). Tag and Release requires
// that MODULE.bazel has none, and that integration/MODULE.bazel has none
// but its local_path_override of this module, which the registry presubmit
// replaces with the release.
//
// buildifier writes each call as `name(` at the start of a line, which
// Check relies on; CI fails on unformatted files.
package nooverrides

import (
	"fmt"
	"regexp"
	"strings"
)

// Override is one override call.
type Override struct {
	Kind   string // archive_override, git_override, ...
	Module string // its module_name, or "" if it has none
	Line   int    // where the call starts, from 1
}

var (
	call       = regexp.MustCompile(`^\s*((?:archive|git|local_path|single_version|multiple_version)_override)\(`)
	moduleName = regexp.MustCompile(`\bmodule_name\s*=\s*"([^"]*)"`)
)

// Find returns the override calls of a MODULE.bazel, in order. A call's
// module_name may be on any line of the call, up to the line that closes
// it; comments are skipped.
func Find(src string) []Override {
	var out []Override
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		m := call.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		o := Override{Kind: m[1], Line: i + 1}
		depth := 0
		for j := i; j < len(lines); j++ {
			l := lines[j]
			if k := strings.Index(l, "#"); k >= 0 {
				l = l[:k]
			}
			if n := moduleName.FindStringSubmatch(l); n != nil && o.Module == "" {
				o.Module = n[1]
			}
			depth += strings.Count(l, "(") - strings.Count(l, ")")
			if depth <= 0 {
				i = j
				break
			}
		}
		out = append(out, o)
	}
	return out
}

// Check returns a problem for each override in src that is not allowed. A
// local_path_override of allowLocal is allowed once; with allowLocal set,
// exactly one is required.
func Check(name, src, allowLocal string) []string {
	var problems []string
	local := 0
	for _, o := range Find(src) {
		if allowLocal != "" && o.Kind == "local_path_override" && o.Module == allowLocal {
			local++
			if local == 1 {
				continue
			}
		}
		problems = append(problems, fmt.Sprintf("%s:%d: %s of %q; remove it before a release", name, o.Line, o.Kind, o.Module))
	}
	if allowLocal != "" && local == 0 {
		problems = append(problems, fmt.Sprintf("%s: no local_path_override of %q; the registry presubmit replaces that one", name, allowLocal))
	}
	return problems
}
