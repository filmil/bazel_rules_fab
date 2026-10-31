// SPDX-License-Identifier: Apache-2.0

// Package silk_pairs_test checks the assembly swatch's extra design rule
// (issue #32). The count check: with the vendor's rules alone, DRC finds
// only silk_overlap, each between a footprint's own silkscreen and one of
// its own pads, and exactly as many as the generator counted. The
// known-bad boards: with the extra rule, a label on a pad, and a
// silkscreen line 0.12 mm from one, still fail under the vendor's own
// clearance.
package silk_pairs_test

import (
	"encoding/json"
	"flag"
	"os"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	jlcpcbPairs = flag.String("jlcpcb_pairs", "", "the generator's count for JLCPCB, as a runfiles path")
	jlcpcbDRC   = flag.String("jlcpcb_drc", "", "JLCPCB's DRC report with the vendor's rules alone")
	pcbwayPairs = flag.String("pcbway_pairs", "", "the generator's count for PCBWay")
	pcbwayDRC   = flag.String("pcbway_drc", "", "PCBWay's DRC report with the vendor's rules alone")
	jlcpcbLabel = flag.String("jlcpcb_label", "", "JLCPCB's label_on_pad board's DRC report")
	pcbwayLabel = flag.String("pcbway_label", "", "PCBWay's label_on_pad board's DRC report")
	jlcpcbNear  = flag.String("jlcpcb_near", "", "JLCPCB's silk_near_pad board's DRC report")
	pcbwayNear  = flag.String("pcbway_near", "", "PCBWay's silk_near_pad board's DRC report")
)

type report struct {
	Violations []struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Items       []struct {
			Description string `json:"description"`
		} `json:"items"`
	} `json:"violations"`
}

func read(t *testing.T, rpath string, v any) {
	t.Helper()
	p, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", rpath, err)
	}
}

// KiCad names a footprint's items "... of <ref> on <layer>".
var (
	silkItem = regexp.MustCompile(` of (\S+) on F\.Silkscreen$`)
	padItem  = regexp.MustCompile(`^Pad \S+ \[[^]]*\] of (\S+) on F\.Cu$`)
)

func check(t *testing.T, vendor, pairs, drc string) {
	var count struct {
		Pairs int `json:"own_silk_pairs"`
	}
	read(t, pairs, &count)
	var report report
	read(t, drc, &report)
	if count.Pairs == 0 {
		t.Errorf("%s: the generator counted no pairs; the extra rule would answer for nothing", vendor)
	}
	n := 0
	for _, v := range report.Violations {
		if v.Type != "silk_overlap" {
			t.Errorf("%s: a %s violation with the vendor's rules alone; only silk_overlap is expected", vendor, v.Type)
			continue
		}
		n++
		if len(v.Items) != 2 {
			t.Errorf("%s: silk_overlap with %d items", vendor, len(v.Items))
			continue
		}
		s := silkItem.FindStringSubmatch(v.Items[0].Description)
		p := padItem.FindStringSubmatch(v.Items[1].Description)
		if s == nil || p == nil || s[1] != p[1] {
			t.Errorf("%s: silk_overlap between %q and %q is not a footprint's silkscreen against its own pad", vendor, v.Items[0].Description, v.Items[1].Description)
		}
	}
	if n != count.Pairs {
		t.Errorf("%s: DRC with the vendor's rules alone finds %d silk_overlap, the generator counted %d pairs", vendor, n, count.Pairs)
	}
	t.Logf("%s: %d pairs of a footprint's own silkscreen and pad, as DRC finds them", vendor, n)
}

func TestJLCPCB(t *testing.T) { check(t, "jlcpcb", *jlcpcbPairs, *jlcpcbDRC) }

func TestPCBWay(t *testing.T) { check(t, "pcbway", *pcbwayPairs, *pcbwayDRC) }

// knownBad wants a silk_overlap between one of the board's own silkscreen
// items (item, such as "PCB text") and a pad, under the vendor's
// clearance (its rule's name holds "Pad to Silkscreen"), and only
// silkscreen violations.
func knownBad(t *testing.T, name, drc, item string) {
	var r report
	read(t, drc, &r)
	found := false
	for _, v := range r.Violations {
		switch v.Type {
		case "silk_overlap", "silk_over_copper":
		default:
			t.Errorf("%s: a %s violation; the known-bad board should differ only in its silkscreen", name, v.Type)
		}
		if v.Type != "silk_overlap" || len(v.Items) != 2 {
			continue
		}
		mine, pad := false, false
		for _, it := range v.Items {
			mine = mine || regexp.MustCompile("^"+item+`\b`).MatchString(it.Description)
			pad = pad || padItem.MatchString(it.Description)
		}
		if mine && pad {
			found = true
			if !regexp.MustCompile(`Pad to Silkscreen`).MatchString(v.Description) {
				t.Errorf("%s: %q: not under the vendor's pad-to-silkscreen rule; the extra rule reaches past footprints' own silkscreen", name, v.Description)
			}
		}
	}
	if !found {
		t.Errorf("%s: DRC found no silk_overlap between the board's %s and a pad", name, item)
	}
}

func TestLabelOnPad(t *testing.T) {
	knownBad(t, "jlcpcb_label_on_pad", *jlcpcbLabel, "PCB text")
	knownBad(t, "pcbway_label_on_pad", *pcbwayLabel, "PCB text")
}

func TestSilkNearPad(t *testing.T) {
	knownBad(t, "jlcpcb_silk_near_pad", *jlcpcbNear, "Segment on F.Silkscreen")
	knownBad(t, "pcbway_silk_near_pad", *pcbwayNear, "Segment on F.Silkscreen")
}
