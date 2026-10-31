// SPDX-License-Identifier: Apache-2.0

// Command assembly writes the assembly swatch's schematic, and its board
// and project (issue #32):
//
//	assembly --spec spec.json --lib Device=Device.kicad_sym ... --sch out.kicad_sch \
//	    --fp Resistor_SMD:R_0402_1005Metric=R_0402_1005Metric.kicad_mod ... \
//	    --pcb out.kicad_pcb --pro out.kicad_pro \
//	    --vendor_rules vendor.kicad_dru --rules board.kicad_dru \
//	    --silk_pairs pairs.json [--bad label_on_pad|silk_near_pad]
//
// The board's rules are the vendor's, as they are, and one rule after
// them (assembly.OwnSilkRule); pairs.json holds the count of the
// silkscreen pairs that rule answers for.
//
// The spec is the ladder of parts that fab/private/assembly.bzl picks for
// the vendor; --lib gives each KiCad symbol library the parts use, and
// --fp each footprint, the test point's among them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/assembly"
	"github.com/filmil/bazel_rules_fab/fab/private/swatch"
)

type libs []string

func (l *libs) String() string     { return strings.Join(*l, ",") }
func (l *libs) Set(v string) error { *l = append(*l, v); return nil }

func run() error {
	specPath := flag.String("spec", "", "the ladder, as JSON")
	out := flag.String("sch", "", "where to write the schematic")
	pcbPath := flag.String("pcb", "", "where to write the board")
	proPath := flag.String("pro", "", "where to write the project")
	vendorRules := flag.String("vendor_rules", "", "the vendor's design rules (.kicad_dru)")
	rulesPath := flag.String("rules", "", "where to write the board's design rules")
	pairsPath := flag.String("silk_pairs", "", "where to write the count of own-silkscreen pairs")
	bad := flag.String("bad", "", "a known-bad variant of the board: label_on_pad or silk_near_pad")
	var ls, fs libs
	flag.Var(&ls, "lib", "NICKNAME=PATH of a .kicad_sym; repeat it")
	flag.Var(&fs, "fp", "LIB:NAME=PATH of a .kicad_mod; repeat it")
	flag.Parse()
	if *specPath == "" || *out == "" || len(ls) == 0 || flag.NArg() > 0 {
		return fmt.Errorf("want --spec, --sch and --lib, and no other arguments")
	}
	if *pcbPath == "" || *proPath == "" || len(fs) == 0 || *vendorRules == "" || *rulesPath == "" || *pairsPath == "" {
		return fmt.Errorf("want --pcb, --pro, --fp, --vendor_rules, --rules and --silk_pairs")
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
	if base(*pcbPath, ".kicad_pcb") != spec.Project || base(*proPath, ".kicad_pro") != spec.Project || base(*out, ".kicad_sch") != spec.Project {
		return fmt.Errorf("--sch, --pcb and --pro must be named after the project %q: KiCad pairs them by name", spec.Project)
	}
	fps := assembly.Footprints{}
	for _, f := range fs {
		id, path, ok := strings.Cut(f, "=")
		if !ok || !strings.Contains(id, ":") {
			return fmt.Errorf("--fp %q: want LIB:NAME=PATH", f)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fp, err := assembly.ReadFootprint(string(text))
		if err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
		fps[id] = fp
	}
	vr, err := os.ReadFile(*vendorRules)
	if err != nil {
		return err
	}
	caps, err := swatch.ParseCaps(string(vr))
	if err != nil {
		return fmt.Errorf("%s: %v", *vendorRules, err)
	}
	sch, pcb, pairs, err := assembly.Design(spec, libraries, fps, assembly.BoardOptions{SilkClear: caps.SilkClear, Bad: *bad})
	if err != nil {
		return err
	}
	count, err := json.Marshal(map[string]int{"own_silk_pairs": pairs})
	if err != nil {
		return err
	}
	for _, w := range []struct{ path, text string }{
		{*out, sch},
		{*pcbPath, pcb},
		{*proPath, assembly.Project(spec.Project)},
		{*rulesPath, string(vr) + assembly.OwnSilkRule},
		{*pairsPath, string(count) + "\n"},
	} {
		if err := os.WriteFile(w.path, []byte(w.text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func base(path, ext string) string {
	return strings.TrimSuffix(filepath.Base(path), ext)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "assembly:", err)
		os.Exit(1)
	}
}
