// SPDX-License-Identifier: Apache-2.0

package swatch

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	// kicad-druid's default files: 4 layers, 1 oz.
	jlcpcb = flag.String("jlcpcb", "", "runfiles path of kicad-druid's JLCPCB.kicad_dru")
	pcbway = flag.String("pcbway", "", "runfiles path of kicad-druid's PCBWay.kicad_dru")
)

func read(t *testing.T, rlocation string) string {
	t.Helper()
	p, err := runfiles.Rlocation(rlocation)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseCaps(t *testing.T) {
	for _, c := range []struct {
		path string
		want Caps
	}{
		{*jlcpcb, Caps{
			Vendor: "JLCPCB", TrackWidth: 0.09, TrackSpacing: 0.09, EdgeClear: 0.2,
			ViaHole: 0.15, ViaAnnular: 0.05, SmallViaHole: 0.3, SmallViaPad: 0.45,
			PTHHole: 0.15, PTHAnnular: 0.15, NPTHHole: 0.5, PadSpacing: 0.15, SMDPad: 0.125,
			TextHeight: 1, TextThickness: 0.15, SilkClear: 0.15,
		}},
		{*pcbway, Caps{
			// The outer layers' track rules: the swatch has two layers.
			Vendor: "PCBWay", TrackWidth: 0.09, TrackSpacing: 0.09, EdgeClear: 0.3,
			ViaHole: 0.2, ViaAnnular: 0.15,
			PTHHole: 0.2, PTHAnnular: 0.15, NPTHHole: 0.5, PadSpacing: 0.127,
			TextHeight: 0.8, TextThickness: 0.15, SilkClear: 0.15,
		}},
	} {
		got, err := ParseCaps(read(t, c.path))
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.path, got, c.want)
		}
	}
}

func TestParseCapsMissing(t *testing.T) {
	_, err := ParseCaps("(version 1)\n(rule \"X: Trace Width\"\n\t(constraint track_width (min 0.1mm))\n)\n")
	if err == nil || !strings.Contains(err.Error(), "the X rules lack \"Trace Spacing\" (clearance)") {
		t.Errorf("got %v", err)
	}
	if _, err := ParseCaps("(version 1)\n"); err == nil {
		t.Error("rules without a vendor: want an error")
	}
}

func TestViaPad(t *testing.T) {
	c := Caps{ViaAnnular: 0.05, SmallViaHole: 0.3, SmallViaPad: 0.45}
	for hole, want := range map[float64]float64{0.15: 0.45, 0.25: 0.45, 0.3: 0.4, 0.5: 0.6} {
		if got := c.ViaPad(hole); got != want {
			t.Errorf("ViaPad(%v) = %v, want %v", hole, got, want)
		}
	}
}

func TestLadderAndLabels(t *testing.T) {
	got := ladder(0.127, []float64{0.1, 0.15, 0.2, 0.25}, 3)
	if len(got) != 3 || got[0] != 0.127 || got[1] != 0.15 || got[2] != 0.2 {
		t.Errorf("ladder: %v", got)
	}
	for v, want := range map[float64]string{0.1: "0.10", 1: "1.00", 0.127: "0.127", 2.5: "2.50"} {
		if got := label(v); got != want {
			t.Errorf("label(%v) = %q, want %q", v, got, want)
		}
	}
	if got := under(0.1); got != 0.08 {
		t.Errorf("under(0.1) = %v", got)
	}
}

func TestBoard(t *testing.T) {
	caps, err := ParseCaps(read(t, *jlcpcb))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Board(caps, Options{Stackup: "2L-1oz"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Board(caps, Options{Stackup: "2L-1oz"})
	if a != b {
		t.Error("two boards from the same rules differ")
	}
	for _, want := range []string{
		`(gr_text "JLCPCB capability swatch, 2L-1oz"`,
		"(width 0.09)",          // the narrowest track
		"(drill 0.15)",          // the smallest via drill
		"(size 0.45)",           // its pad, by the extra-cost rule
		`(gr_text "0.09"`,       // a label
		`(layer "Edge.Cuts")`,   // the outline
		"(start 23.18 20)",      // which starts after the top left corner
	} {
		if !strings.Contains(a, want) {
			t.Errorf("the board lacks %q", want)
		}
	}
	if strings.Count(a, "(") != strings.Count(a, ")") {
		t.Error("unbalanced parentheses")
	}
	for f := range Features {
		bad, err := Board(caps, Options{Below: f})
		if err != nil {
			t.Fatal(err)
		}
		if bad == a {
			t.Errorf("below %s: same board as the real one", f)
		}
	}
	if _, err := Board(caps, Options{Below: "colour"}); err == nil || !strings.Contains(err.Error(), "want one of edge,") {
		t.Errorf("unknown feature: %v", err)
	}
}

func TestProject(t *testing.T) {
	p := Project("s")
	for _, want := range []string{`"filename": "s.kicad_pro"`, `"track_dangling": "ignore"`, `"min_track_width": 0.0`} {
		if !strings.Contains(p, want) {
			t.Errorf("the project lacks %q", want)
		}
	}
}
