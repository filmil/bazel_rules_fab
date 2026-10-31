// SPDX-License-Identifier: Apache-2.0

// Command swatch writes a fab house's capability swatch (issue #24): a
// .kicad_pcb and its .kicad_pro, from the vendor's KiCad design rules.
//
//	swatch --rules JLCPCB-2L-1oz.kicad_dru --stackup 2L-1oz \
//	    --pcb swatch.kicad_pcb --pro swatch.kicad_pro [--net swatch.net] \
//	    [--below track_width]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/swatch"
)

func run() error {
	rules := flag.String("rules", "", "the vendor's design rules (.kicad_dru)")
	stackup := flag.String("stackup", "", "the stackup, for the title")
	pcb := flag.String("pcb", "", "where to write the board (.kicad_pcb)")
	pro := flag.String("pro", "", "where to write the project (.kicad_pro)")
	net := flag.String("net", "", "where to write the empty netlist (.net), if anywhere")
	below := flag.String("below", "", "a feature to draw under its minimum, for a known-bad board")
	flag.Parse()
	if *rules == "" || *pcb == "" || *pro == "" || flag.NArg() > 0 {
		return fmt.Errorf("want --rules, --pcb and --pro, and no other arguments")
	}
	if strings.TrimSuffix(filepath.Base(*pcb), ".kicad_pcb") != strings.TrimSuffix(filepath.Base(*pro), ".kicad_pro") {
		return fmt.Errorf("--pcb and --pro must share a name: KiCad pairs them by name")
	}
	text, err := os.ReadFile(*rules)
	if err != nil {
		return err
	}
	caps, err := swatch.ParseCaps(string(text))
	if err != nil {
		return fmt.Errorf("%s: %v", *rules, err)
	}
	board, err := swatch.Board(caps, swatch.Options{Stackup: *stackup, Below: *below})
	if err != nil {
		return err
	}
	if err := os.WriteFile(*pcb, []byte(board), 0o644); err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(*pro), ".kicad_pro")
	if err := os.WriteFile(*pro, []byte(swatch.Project(name)), 0o644); err != nil {
		return err
	}
	if *net != "" {
		return os.WriteFile(*net, []byte(swatch.Netlist), 0o644)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "swatch:", err)
		os.Exit(1)
	}
}
