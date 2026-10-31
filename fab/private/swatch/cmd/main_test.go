// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rules = `(version 1)
(rule "V: Trace Width"
	(constraint track_width (min 0.1mm))
)
(rule "V: Trace Spacing"
	(constraint clearance (min 0.1mm))
)
(rule "V: Trace to Board Edge"
	(constraint edge_clearance (min 0.2mm))
)
(rule "V: Via Hole Size"
	(constraint hole_size (min 0.2mm))
)
(rule "V: Via Annular Ring"
	(constraint annular_width (min 0.1mm))
)
(rule "V: PTH Hole Size"
	(constraint hole_size (min 0.2mm))
)
(rule "V: PTH Annular Ring"
	(constraint annular_width (min 0.15mm))
)
(rule "V: NPTH Hole Size"
	(constraint hole_size (min 0.5mm))
)
(rule "V: Pad to Pad Clearance (Pad without Hole, Different Nets)"
	(constraint clearance (min 0.15mm))
)
(rule "V: Minimum Text Height"
	(constraint text_height (min 1mm))
)
(rule "V: Minimum Line Width"
	(constraint text_thickness (min 0.15mm))
)
(rule "V: Pad to Silkscreen"
	(constraint silk_clearance (min 0.15mm))
)
`

func runWith(t *testing.T, args ...string) error {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet("swatch", flag.ContinueOnError)
	os.Args = append([]string{"swatch"}, args...)
	return run()
}

func TestRun(t *testing.T) {
	d := t.TempDir()
	dru := filepath.Join(d, "v.kicad_dru")
	if err := os.WriteFile(dru, []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	pcb, pro, net := filepath.Join(d, "s.kicad_pcb"), filepath.Join(d, "s.kicad_pro"), filepath.Join(d, "s.net")
	if err := runWith(t, "--rules", dru, "--stackup", "2L-1oz", "--pcb", pcb, "--pro", pro, "--net", net); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{pcb, pro, net} {
		if b, err := os.ReadFile(f); err != nil || len(b) == 0 {
			t.Errorf("%s: %v, %d bytes", f, err, len(b))
		}
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--rules", dru, "--pcb", pcb}, "want --rules, --pcb and --pro"},
		{[]string{"--rules", dru, "--pcb", pcb, "--pro", filepath.Join(d, "t.kicad_pro")}, "must share a name"},
		{[]string{"--rules", dru, "--pcb", pcb, "--pro", pro, "extra"}, "no other arguments"},
		{[]string{"--rules", filepath.Join(d, "none"), "--pcb", pcb, "--pro", pro}, "no such file"},
		{[]string{"--rules", dru, "--pcb", pcb, "--pro", pro, "--below", "x"}, "want one of"},
	} {
		if err := runWith(t, c.args...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
}
