// SPDX-License-Identifier: Apache-2.0

package swatch

import (
	"flag"
	"fmt"
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
			// One track rule for every layer: inner as outer.
			Vendor: "JLCPCB", TrackWidth: 0.09, TrackSpacing: 0.09, EdgeClear: 0.2,
			InnerTrackWidth: 0.09, InnerTrackSpacing: 0.09,
			ViaHole: 0.15, ViaAnnular: 0.05, SmallViaHole: 0.3, SmallViaPad: 0.45,
			PTHHole: 0.15, PTHAnnular: 0.15, NPTHHole: 0.5, PadSpacing: 0.15,
			TextHeight: 1, TextThickness: 0.15, SilkClear: 0.15,
		}},
		{*pcbway, Caps{
			// "(Outer Layer)" and "(Inner Layer)" rules of their own.
			Vendor: "PCBWay", TrackWidth: 0.09, TrackSpacing: 0.09, EdgeClear: 0.3,
			InnerTrackWidth: 0.1, InnerTrackSpacing: 0.1,
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
	twice := "(rule \"X: Trace Width\"\n\t(constraint track_width (min 0.1mm))\n)\n"
	if _, err := ParseCaps(twice + twice); err == nil || !strings.Contains(err.Error(), "appears twice") {
		t.Errorf("a rule twice: %v", err)
	}
	if _, err := ParseCaps(twice + strings.Replace(twice, "X: Trace Width", "Y: Trace Spacing", 1)); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Errorf("two vendors: %v", err)
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
	for v, want := range map[float64]float64{0.1: 0.08, 0.09: 0.072, 0.127: 0.1016, 0.01: 0.008} {
		if got := under(v); got != want {
			t.Errorf("under(%v) = %v, want %v", v, got, want)
		}
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
		"(width 0.09)",        // the narrowest track
		"(drill 0.15)",        // the smallest via drill
		"(size 0.45)",         // its pad, by the extra-cost rule
		`(gr_text "0.09"`,     // a label
		`(layer "Edge.Cuts")`, // the outline
		"(start 23.18 20)",    // which starts after the top left corner
	} {
		if !strings.Contains(a, want) {
			t.Errorf("the board lacks %q", want)
		}
	}
	if strings.Count(a, "(") != strings.Count(a, ")") {
		t.Error("unbalanced parentheses")
	}
	for f := range Features {
		o := Options{Below: f}
		if strings.HasPrefix(f, "inner_") {
			o.Stackup = "4L-1oz"
		}
		bad, err := Board(caps, o)
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
	// The thin ring goes on the 0.30 via, the first that JLCPCB's
	// extra-cost rule does not cover; the drills stay in order.
	ring, _ := Board(caps, Options{Below: "via_annular"})
	if !strings.Contains(ring, "(size 0.38)\n\t\t(drill 0.3)") {
		t.Error("below via_annular: no 0.30 via with a 0.04 ring")
	}
	if strings.Count(ring, "(drill 0.3)\n\t\t(layers \"F.Cu\" \"B.Cu\")") != 1 {
		t.Error("below via_annular: the 0.30 via is not there once")
	}
	// A vendor whose plated hole pads are too big for their row.
	fat := caps
	fat.PTHAnnular = 1.5
	if _, err := Board(fat, Options{}); err == nil || !strings.Contains(err.Error(), "silkscreen clearance") {
		t.Errorf("pads too big: %v", err)
	}
}

func TestInnerLayers(t *testing.T) {
	caps, err := ParseCaps(read(t, *pcbway))
	if err != nil {
		t.Fatal(err)
	}
	two, err := Board(caps, Options{Stackup: "2L-1oz"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(two, "In1.Cu") || strings.Contains(two, "INNER") {
		t.Error("a 2L board has inner layers")
	}
	for _, c := range []struct {
		stackup string
		inner   int
	}{{"4L-1oz", 2}, {"4L-2oz", 2}, {"6L-1oz", 4}} {
		b, err := Board(caps, Options{Stackup: c.stackup})
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 4; i++ {
			name := fmt.Sprintf("\"In%d.Cu\" signal", i)
			if got := strings.Contains(b, name); got != (i <= c.inner) {
				t.Errorf("%s: In%d.Cu in the layer table: %v", c.stackup, i, got)
			}
		}
		for _, want := range []string{
			`(gr_text "INNER WIDTH (In1)"`,
			"(width 0.1)\n\t\t(layer \"In1.Cu\")", // PCBWay's inner minimum, not the outer 0.09
			`(gr_text "INNER SPACING (In1)"`,
		} {
			if !strings.Contains(b, want) {
				t.Errorf("%s: the board lacks %q", c.stackup, want)
			}
		}
	}
	bad, err := Board(caps, Options{Stackup: "6L-1oz", Below: "inner_track_width"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bad, "(width 0.08)\n\t\t(layer \"In1.Cu\")") {
		t.Error("below inner_track_width: no 0.08 track on In1.Cu")
	}
	for _, f := range []string{"inner_track_width", "inner_track_spacing"} {
		if _, err := Board(caps, Options{Stackup: "2L-1oz", Below: f}); err == nil || !strings.Contains(err.Error(), "4 layers or more") {
			t.Errorf("%s on 2L: %v", f, err)
		}
	}
	// An inner spacing under PCBWay's 0.1 mm inner minimum: 0.08.
	gap, err := Board(caps, Options{Stackup: "4L-1oz", Below: "inner_track_spacing"})
	if err != nil {
		t.Fatal(err)
	}
	if gap == bad {
		t.Error("below inner_track_spacing: no change")
	}
	if !strings.Contains(gap, `(gr_text "0.08"`) {
		t.Error("below inner_track_spacing: no 0.08 label")
	}
	// Rules with outer track rules and no inner ones (a 2-layer stackup's)
	// make a 2-layer swatch, and no 4-layer one: the outer minimums say
	// nothing of inner layers.
	outer := caps
	outer.InnerTrackWidth, outer.InnerTrackSpacing = 0, 0
	if _, err := Board(outer, Options{Stackup: "2L-1oz"}); err != nil {
		t.Errorf("2L with outer rules only: %v", err)
	}
	if _, err := Board(outer, Options{Stackup: "4L-1oz"}); err == nil || !strings.Contains(err.Error(), "no inner track width and spacing") {
		t.Errorf("4L with outer rules only: %v", err)
	}
	for _, s := range []string{"3L-1oz", "L-1oz", "4X-1oz", "four", "4L", "4L-", "34L-1oz"} {
		if _, err := Board(caps, Options{Stackup: s}); err == nil || !strings.Contains(err.Error(), "even number of copper layers") {
			t.Errorf("stackup %q: %v", s, err)
		}
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
