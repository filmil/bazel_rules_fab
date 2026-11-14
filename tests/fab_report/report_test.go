// SPDX-License-Identifier: Apache-2.0

// Package report_test reads the reports fab_report writes for the
// hand-written board.
package report_test

import (
	"bytes"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	jlcpcb = flag.String("jlcpcb", "", "the JLCPCB package's report, as a runfiles path")
	pcbway = flag.String("pcbway", "", "the PCBWay package's report")
	bare   = flag.String("bare", "", "the bare board's report")
	empty  = flag.String("empty", "", "the report of a package of empty files")
)

func dir(t *testing.T, rel string) string {
	t.Helper()
	p, err := runfiles.Rlocation(rel)
	if err != nil {
		t.Fatalf("runfiles %s: %v", rel, err)
	}
	return p
}

func read(t *testing.T, d, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func has(t *testing.T, what string, md []byte, want ...string) {
	t.Helper()
	for _, w := range want {
		if !bytes.Contains(md, []byte(w)) {
			t.Errorf("%s has no %q:\n%s", what, w, md)
		}
	}
}

var layers = []string{
	"main-F_Cu.gtl", "main-B_Cu.gbl", "main-F_Mask.gts", "main-B_Mask.gbs",
	"main-F_Silkscreen.gto", "main-B_Silkscreen.gbo", "main-F_Paste.gtp",
	"main-B_Paste.gbp", "main-Edge_Cuts.gm1", "main.drl",
}

func TestJLCPCB(t *testing.T) {
	d := dir(t, *jlcpcb)
	has(t, "jlcpcb report.md", read(t, d, "report.md"),
		"# jlcpcb.zip",
		"* Size: 25.00 x 15.00 mm (the outline's box).",
		"* Copper layers: 2.",
		"| `main.drl` | T3 | 2.200 | NPTH | 2 | 0 |",
		"| `main.drl` | T4 | 0.800 | NPTH | 0 | 1 |",
		"5 holes and 1 slot; the smallest drill is 0.300 mm.",
		"| LED | D1 | LED_0603_1608Metric | C2286 |",
		"`main_bom.csv`: 3 lines, 3 parts.",
		"| Top | 2 |\n| Bottom | 1 |",
		"Result: no problems.",
	)
	var names []string
	for _, l := range layers {
		names = append(names, l+".png")
	}
	// 25 x 15 mm and a 1 mm margin, at 20 pixels per mm.
	for _, n := range append(names, "top.png", "bottom.png") {
		img, err := png.Decode(bytes.NewReader(read(t, d, n)))
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != 540 || b.Dy() != 340 {
			t.Errorf("%s: %dx%d pixels, want 540x340", n, b.Dx(), b.Dy())
		}
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(layers)+3 {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Errorf("files %s; want report.md, top.png, bottom.png and a PNG per layer", strings.Join(got, ", "))
	}
}

func TestPCBWay(t *testing.T) {
	has(t, "pcbway report.md", read(t, dir(t, *pcbway), "report.md"),
		"The fab package for pcbway",
		"| 1 | D1 | 1 | Lite-On | LTST-C191KRKT | LED | LED_0603_1608Metric |  |  |",
		"| Side (`TB`) | Parts |",
		"| T | 2 |\n| B | 1 |",
		"Result: no problems.",
	)
}

func TestBare(t *testing.T) {
	md := read(t, dir(t, *bare), "report.md")
	has(t, "bare report.md", md, "A bare board: no BOM and no placement list.", "Result: no problems.")
	if bytes.Contains(md, []byte("## BOM")) {
		t.Errorf("a bare board's report has a BOM:\n%s", md)
	}
}

func TestEmpty(t *testing.T) {
	has(t, "empty report.md", read(t, dir(t, *empty), "report.md"),
		"Nothing to draw: the outline and every layer are empty.")
}
