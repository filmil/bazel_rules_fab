// SPDX-License-Identifier: Apache-2.0

// Command assembly writes the assembly swatch's schematic (issue #32):
//
//	assembly --spec spec.json --lib Device=Device.kicad_sym ... --sch out.kicad_sch
//
// The spec is the ladder of parts that fab/private/assembly.bzl picks for
// the vendor; --lib gives each KiCad symbol library the parts use.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/assembly"
)

type libs []string

func (l *libs) String() string     { return strings.Join(*l, ",") }
func (l *libs) Set(v string) error { *l = append(*l, v); return nil }

func run() error {
	specPath := flag.String("spec", "", "the ladder, as JSON")
	out := flag.String("sch", "", "where to write the schematic")
	var ls libs
	flag.Var(&ls, "lib", "NICKNAME=PATH of a .kicad_sym; repeat it")
	flag.Parse()
	if *specPath == "" || *out == "" || len(ls) == 0 || flag.NArg() > 0 {
		return fmt.Errorf("want --spec, --sch and --lib, and no other arguments")
	}
	b, err := os.ReadFile(*specPath)
	if err != nil {
		return err
	}
	var spec assembly.Spec
	if err := json.Unmarshal(b, &spec); err != nil {
		return fmt.Errorf("%s: %v", *specPath, err)
	}
	libraries := assembly.Libraries{}
	for _, l := range ls {
		name, path, ok := strings.Cut(l, "=")
		if !ok {
			return fmt.Errorf("--lib %q: want NICKNAME=PATH", l)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lib, err := assembly.ReadLibrary(name, string(text))
		if err != nil {
			return err
		}
		libraries[name] = lib
	}
	sch, err := assembly.Schematic(spec, libraries)
	if err != nil {
		return err
	}
	return os.WriteFile(*out, []byte(sch), 0o644)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "assembly:", err)
		os.Exit(1)
	}
}
