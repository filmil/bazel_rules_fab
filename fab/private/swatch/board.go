// SPDX-License-Identifier: Apache-2.0

// Package swatch draws a fab house's capability swatch (issue #24): a
// bare two-layer board the size of a credit card (ISO/IEC 7810 ID-1,
// 85.60 x 53.98 mm, corners of 3.18 mm), with ladders of the vendor's
// smallest features, each labelled with its size on the silkscreen.
//
// The numbers come from the vendor's KiCad design rules (ParseCaps), so
// the board passes DRC with those rules. Below makes a known-bad variant:
// one feature a step under its minimum.
package swatch

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// The card, mm.
const (
	CardW      = 85.60
	CardH      = 53.98
	CardRadius = 3.18
	// Where the card's top left corner sits on KiCad's page.
	originX, originY = 20.0, 20.0
	// Labels and headings: tall enough for every vendor's minimum.
	labelSize  = 1.0
	labelThick = 0.15
)

// Features that Below can push under the minimum, and the DRC violation
// each then gives.
var Features = map[string]string{
	"track_width":   "track_width",
	"track_spacing": "clearance",
	"via_hole":      "drill_out_of_range",
	"via_annular":   "annular_width",
	"edge":          "copper_edge_clearance",
	"text_height":   "text_height",
}

// Options says what to draw.
type Options struct {
	Stackup string // shown in the title, such as "2L-1oz"
	// Below names a feature (a key of Features) to draw a step under its
	// minimum, for a known-bad board; empty for the real swatch.
	Below string
}

func round(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func num(v float64) string { return strconv.FormatFloat(round(v), 'f', -1, 64) }

// label is how a size reads on the board: two decimals at least.
func label(v float64) string {
	s := strconv.FormatFloat(round(v), 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i < 0 {
		s += ".00"
	} else if len(s)-i == 2 {
		s += "0"
	}
	return s
}

// under is a step under a minimum: 80 % of it.
func under(v float64) float64 { return round(v * 0.8) }

// ladder is the minimum, then each step above it, at most n values.
func ladder(min float64, steps []float64, n int) []float64 {
	out := []float64{round(min)}
	for _, s := range steps {
		if len(out) == n {
			break
		}
		if s > min+1e-9 {
			out = append(out, s)
		}
	}
	return out
}

type board struct {
	b    strings.Builder
	nets []string
	n    int // items drawn, for their UUIDs
}

// uuid is a fixed UUID for the n-th item, so the board's bytes repeat.
func (bd *board) uuid() string {
	bd.n++
	h := sha256.Sum256([]byte(fmt.Sprintf("hdlf_rules_fab swatch %d", bd.n)))
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// net makes a net of its own for one item: items of different nets are
// what the clearance rules measure between.
func (bd *board) net() int {
	bd.nets = append(bd.nets, fmt.Sprintf("S%d", len(bd.nets)+1))
	return len(bd.nets)
}

func xy(x, y float64) string { return num(originX+x) + " " + num(originY+y) }

func (bd *board) f(format string, a ...any) { fmt.Fprintf(&bd.b, format, a...) }

func (bd *board) text(s string, x, y, size, thick float64, layer, justify string) {
	j := ""
	if justify != "" {
		j = " (justify " + justify + ")"
	}
	bd.f("\t(gr_text %q\n\t\t(at %s 0)\n\t\t(layer %q)\n\t\t(uuid %q)\n\t\t(effects\n\t\t\t(font\n\t\t\t\t(size %s %s)\n\t\t\t\t(thickness %s)\n\t\t\t)%s\n\t\t)\n\t)\n",
		s, xy(x, y), layer, bd.uuid(), num(size), num(size), num(thick), j)
}

func (bd *board) label(s string, x, y float64) { bd.text(s, x, y, labelSize, labelThick, "F.SilkS", "") }

func (bd *board) heading(s string, x, y float64) {
	bd.text(s, x, y, labelSize, labelThick, "F.SilkS", "left")
}

func (bd *board) line(x1, y1, x2, y2, width float64, layer string) {
	bd.f("\t(gr_line\n\t\t(start %s)\n\t\t(end %s)\n\t\t(stroke\n\t\t\t(width %s)\n\t\t\t(type solid)\n\t\t)\n\t\t(layer %q)\n\t\t(uuid %q)\n\t)\n",
		xy(x1, y1), xy(x2, y2), num(width), layer, bd.uuid())
}

func (bd *board) arc(x1, y1, xm, ym, x2, y2 float64) {
	bd.f("\t(gr_arc\n\t\t(start %s)\n\t\t(mid %s)\n\t\t(end %s)\n\t\t(stroke\n\t\t\t(width 0.05)\n\t\t\t(type solid)\n\t\t)\n\t\t(layer \"Edge.Cuts\")\n\t\t(uuid %q)\n\t)\n",
		xy(x1, y1), xy(xm, ym), xy(x2, y2), bd.uuid())
}

func (bd *board) track(x1, y1, x2, y2, width float64, layer string) {
	bd.f("\t(segment\n\t\t(start %s)\n\t\t(end %s)\n\t\t(width %s)\n\t\t(layer %q)\n\t\t(net %d)\n\t\t(uuid %q)\n\t)\n",
		xy(x1, y1), xy(x2, y2), num(width), layer, bd.net(), bd.uuid())
}

func (bd *board) via(x, y, pad, hole float64) {
	bd.f("\t(via\n\t\t(at %s)\n\t\t(size %s)\n\t\t(drill %s)\n\t\t(layers \"F.Cu\" \"B.Cu\")\n\t\t(net %d)\n\t\t(uuid %q)\n\t)\n",
		xy(x, y), num(pad), num(hole), bd.net(), bd.uuid())
}

// pad is a footprint with one pad: kind "thru_hole", "np_thru_hole" or
// "smd"; w and h its size, hole its drill (0 for an SMD pad).
func (bd *board) pad(ref, kind string, x, y, w, h, hole float64) {
	shape, layers, attr := "circle", `"*.Cu" "*.Mask"`, "through_hole"
	switch kind {
	case "smd":
		shape, layers, attr = "rect", `"F.Cu" "F.Mask"`, "smd"
	case "np_thru_hole":
		attr = "exclude_from_pos_files exclude_from_bom"
	}
	drill, net := "", ""
	if hole > 0 {
		drill = fmt.Sprintf("\n\t\t\t(drill %s)", num(hole))
	}
	if kind != "np_thru_hole" {
		n := bd.net()
		net = fmt.Sprintf("\n\t\t\t(net %d %q)", n, bd.nets[n-1])
	}
	bd.f("\t(footprint \"swatch:%s\"\n\t\t(layer \"F.Cu\")\n\t\t(uuid %q)\n\t\t(at %s)\n", kind, bd.uuid(), xy(x, y))
	for _, p := range []struct{ k, v string }{{"Reference", ref}, {"Value", kind}} {
		bd.f("\t\t(property %q %q\n\t\t\t(at 0 0 0)\n\t\t\t(layer \"F.Fab\")\n\t\t\t(hide yes)\n\t\t\t(uuid %q)\n\t\t\t(effects\n\t\t\t\t(font\n\t\t\t\t\t(size 1 1)\n\t\t\t\t\t(thickness 0.15)\n\t\t\t\t)\n\t\t\t)\n\t\t)\n", p.k, p.v, bd.uuid())
	}
	bd.f("\t\t(attr %s)\n\t\t(pad \"1\" %s %s\n\t\t\t(at 0 0)\n\t\t\t(size %s %s)%s\n\t\t\t(layers %s)%s\n\t\t\t(uuid %q)\n\t\t)\n\t\t(embedded_fonts no)\n\t)\n",
		attr, kind, shape, num(w), num(h), drill, layers, net, bd.uuid())
}

// fits checks that an exposed pad of size `size`, centered at y, keeps
// the vendor's silkscreen clearance from the labels centered at above
// and below (0: none). A label's ink reaches half its height plus half
// its stroke from its center.
func fits(c Caps, what string, size, y, above, below float64) error {
	ink := labelSize/2 + labelThick/2
	gaps := []float64{(y - size/2) - (above + ink)}
	if below > 0 {
		gaps = append(gaps, (below-ink)-(y+size/2))
	}
	for _, g := range gaps {
		if g < c.SilkClear {
			return fmt.Errorf("%s of %s mm comes %s mm from its label, under the %s silkscreen clearance %s mm",
				what, num(size), num(g), c.Vendor, num(c.SilkClear))
		}
	}
	return nil
}

func (bd *board) outline() {
	r := CardRadius
	k := r * (1 - math.Sqrt2/2) // a corner arc's middle, from the corner
	bd.line(r, 0, CardW-r, 0, 0.05, "Edge.Cuts")
	bd.line(CardW, r, CardW, CardH-r, 0.05, "Edge.Cuts")
	bd.line(CardW-r, CardH, r, CardH, 0.05, "Edge.Cuts")
	bd.line(0, CardH-r, 0, r, 0.05, "Edge.Cuts")
	bd.arc(0, r, k, k, r, 0)
	bd.arc(CardW-r, 0, CardW-k, k, CardW, r)
	bd.arc(CardW, CardH-r, CardW-k, CardH-k, CardW-r, CardH)
	bd.arc(r, CardH, k, CardH-k, 0, CardH-r)
}

// Board returns the swatch's .kicad_pcb.
func Board(c Caps, o Options) (string, error) {
	if o.Below != "" {
		if _, ok := Features[o.Below]; !ok {
			var names []string
			for k := range Features {
				names = append(names, k)
			}
			sort.Strings(names)
			return "", fmt.Errorf("below = %q: want one of %s", o.Below, strings.Join(names, ", "))
		}
	}
	low := func(feature string, v float64) float64 {
		if o.Below == feature {
			return under(v)
		}
		return v
	}
	bd := &board{}
	bd.outline()

	// The title.
	title := c.Vendor + " capability swatch"
	if o.Stackup != "" {
		title += ", " + o.Stackup
	}
	bd.text(title, 4, 3.6, 1.5, 0.2, "F.SilkS", "left")
	bd.heading("hdlf_rules_fab: each feature at the vendor's minimum and up, sizes in mm", 4, 5.8)

	// Track widths: vertical tracks, the first at the minimum, on both
	// sides. The back's text is mirrored, so it reads from the back.
	bd.heading("TRACK WIDTH", 4, 8.2)
	bd.text(c.Vendor+" capability swatch, back", CardW/2, 3.6, 1.5, 0.2, "B.SilkS", "mirror")
	bd.text("TRACK WIDTH, BACK", 23, 8.2, labelSize, labelThick, "B.SilkS", "mirror")
	for i, w := range ladder(c.TrackWidth, []float64{0.15, 0.2, 0.25, 0.3, 0.4, 0.5, 0.75, 1.0}, 8) {
		x := 5.5 + float64(i)*5
		if i == 0 {
			w = low("track_width", w)
		}
		bd.label(label(w), x, 10.2)
		bd.text(label(w), x, 10.2, labelSize, labelThick, "B.SilkS", "mirror")
		bd.track(x, 11.8, x, 17.2, w, "F.Cu")
		bd.track(x, 11.8, x, 17.2, w, "B.Cu")
	}

	// Track spacing: pairs of the narrowest tracks.
	bd.heading("TRACK SPACING", 47, 8.2)
	tw := c.TrackWidth
	for i, g := range ladder(c.TrackSpacing, []float64{0.15, 0.2, 0.25, 0.3, 0.5}, 6) {
		x := 49 + float64(i)*6
		if i == 0 {
			g = low("track_spacing", g)
		}
		d := (g + tw) / 2 // from the pair's middle to each track's center line
		bd.label(label(g), x, 10.2)
		bd.track(x-d, 11.8, x-d, 17.2, tw, "F.Cu")
		bd.track(x+d, 11.8, x+d, 17.2, tw, "F.Cu")
	}

	// Vias: drill above, pad below.
	bd.heading("VIA (drill, pad)", 4, 20.2)
	vias := ladder(c.ViaHole, []float64{0.2, 0.25, 0.3, 0.4, 0.5, 0.6}, 7)
	// A thin ring goes on the first via that the extra-cost rule (a
	// bigger pad for a small hole) does not cover, so only the ring rule
	// fires.
	thin := 0
	for thin < len(vias)-1 && c.SmallViaPad > 0 && vias[thin] < c.SmallViaHole {
		thin++
	}
	for i, h := range vias {
		x := 5.5 + float64(i)*5.5
		pad := c.ViaPad(h)
		if i == 0 {
			h = low("via_hole", h)
		}
		if i == thin && o.Below == "via_annular" {
			pad = round(h + 2*under(c.ViaAnnular))
		}
		bd.label(label(h), x, 22.2)
		bd.via(x, 24.6, pad, h)
		bd.label(label(pad), x, 27)
	}

	// Plated through holes: drill above, pad below.
	bd.heading("PLATED HOLE (drill, pad)", 47, 20.2)
	for i, h := range ladder(c.PTHHole, []float64{0.3, 0.5, 0.8, 1.0, 1.2}, 6) {
		x := 49 + float64(i)*6
		pad := round(h + 2*c.PTHAnnular)
		if err := fits(c, "a plated hole's pad", pad, 24.6, 22.2, 27); err != nil {
			return "", err
		}
		bd.label(label(h), x, 22.2)
		bd.pad(fmt.Sprintf("H%d", i+1), "thru_hole", x, 24.6, pad, pad, h)
		bd.label(label(pad), x, 27)
	}

	// Non-plated holes.
	bd.heading("NON-PLATED HOLE", 4, 30.2)
	for i, h := range ladder(c.NPTHHole, []float64{0.8, 1.0, 1.5, 2.0, 2.5}, 6) {
		x := 5.5 + float64(i)*6
		bd.label(label(h), x, 32.2)
		bd.pad(fmt.Sprintf("N%d", i+1), "np_thru_hole", x, 35.4, h, h, h)
	}

	// SMD pads: pairs with the gap between them, which is also the
	// solder mask web (the mask opening is the pad).
	bd.heading("SMD PAD GAP", 47, 30.2)
	for i, g := range ladder(c.PadSpacing, []float64{0.2, 0.25, 0.3, 0.4, 0.5}, 6) {
		x := 49 + float64(i)*6
		d := (g + 0.6) / 2
		if err := fits(c, "an SMD pad", 1.2, 35.4, 32.2, 0); err != nil {
			return "", err
		}
		bd.label(label(g), x, 32.2)
		bd.pad(fmt.Sprintf("P%da", i+1), "smd", x-d, 35.4, 0.6, 1.2, 0)
		bd.pad(fmt.Sprintf("P%db", i+1), "smd", x+d, 35.4, 0.6, 1.2, 0)
	}

	// Silkscreen: text at each height reads its own height; lines at
	// each width.
	bd.heading("SILKSCREEN TEXT", 4, 40.2)
	x := 4.0
	for i, h := range ladder(c.TextHeight, []float64{1.0, 1.25, 1.5, 2.0}, 4) {
		if i == 0 {
			h = low("text_height", h)
		}
		th := math.Max(c.TextThickness, round(h*0.15))
		bd.text(label(h), x, 43.6, h, th, "F.SilkS", "left")
		x += 2.6*h + 2.2
	}
	bd.heading("SILKSCREEN LINE", 4, 47.4)
	for i, w := range ladder(c.TextThickness, []float64{0.2, 0.3, 0.5}, 4) {
		x := 5.5 + float64(i)*7
		bd.line(x-2, 49.4, x+2, 49.4, w, "F.SilkS")
		bd.label(label(w), x, 51.2)
	}

	// Copper to the board edge: a track along the bottom edge, at the
	// minimum distance.
	bd.heading("TRACK TO EDGE", 47, 47.4)
	e := low("edge", c.EdgeClear)
	bd.label(label(e), 60, 50.2)
	y := CardH - e - tw/2
	bd.track(47, y, 73, y, tw, "F.Cu")

	var out strings.Builder
	out.WriteString("(kicad_pcb\n\t(version 20241229)\n\t(generator \"hdlf_rules_fab_swatch\")\n\t(generator_version \"9.0\")\n")
	out.WriteString("\t(general\n\t\t(thickness 1.6)\n\t\t(legacy_teardrops no)\n\t)\n\t(paper \"A4\")\n")
	out.WriteString(layers)
	out.WriteString("\t(setup\n\t\t(pad_to_mask_clearance 0)\n\t\t(allow_soldermask_bridges_in_footprints no)\n\t\t(tenting front back)\n\t)\n")
	out.WriteString("\t(net 0 \"\")\n")
	for i, n := range bd.nets {
		fmt.Fprintf(&out, "\t(net %d %q)\n", i+1, n)
	}
	out.WriteString(bd.b.String())
	out.WriteString("\t(embedded_fonts no)\n)\n")
	return out.String(), nil
}

const layers = `	(layers
		(0 "F.Cu" signal)
		(2 "B.Cu" signal)
		(9 "F.Adhes" user "F.Adhesive")
		(11 "B.Adhes" user "B.Adhesive")
		(13 "F.Paste" user)
		(15 "B.Paste" user)
		(5 "F.SilkS" user "F.Silkscreen")
		(7 "B.SilkS" user "B.Silkscreen")
		(1 "F.Mask" user)
		(3 "B.Mask" user)
		(17 "Dwgs.User" user "User.Drawings")
		(19 "Cmts.User" user "User.Comments")
		(25 "Edge.Cuts" user)
		(27 "Margin" user)
		(31 "F.CrtYd" user "F.Courtyard")
		(29 "B.CrtYd" user "B.Courtyard")
		(35 "F.Fab" user)
		(33 "B.Fab" user)
	)
`

// Netlist is the swatch's netlist: empty, as it has no parts. kicad_pcb
// takes a board with a schematic or a netlist.
const Netlist = "(export\n\t(version \"E\")\n\t(components)\n\t(nets)\n)\n"

// Project returns the swatch's .kicad_pro. The board's own minimums are
// zero, so the vendor's rules alone decide; dangling tracks and vias (a
// swatch connects nothing) and footprints that are in no library are not
// checked.
func Project(name string) string {
	return fmt.Sprintf(`{
  "board": {
    "design_settings": {
      "rule_severities": {
        "footprint": "ignore",
        "lib_footprint_issues": "ignore",
        "lib_footprint_mismatch": "ignore",
        "missing_courtyard": "ignore",
        "track_dangling": "ignore",
        "via_dangling": "ignore"
      },
      "rules": {
        "max_error": 0.005,
        "min_clearance": 0.0,
        "min_connection": 0.0,
        "min_copper_edge_clearance": 0.0,
        "min_groove_width": 0.0,
        "min_hole_clearance": 0.0,
        "min_hole_to_hole": 0.0,
        "min_microvia_diameter": 0.0,
        "min_microvia_drill": 0.0,
        "min_resolved_spokes": 2,
        "min_silk_clearance": 0.0,
        "min_text_height": 0.0,
        "min_text_thickness": 0.0,
        "min_through_hole_diameter": 0.0,
        "min_track_width": 0.0,
        "min_via_annular_width": 0.0,
        "min_via_diameter": 0.0,
        "solder_mask_to_copper_clearance": 0.0
      }
    }
  },
  "meta": {
    "filename": %q,
    "version": 3
  },
  "net_settings": {
    "classes": [
      {
        "clearance": 0.0,
        "name": "Default",
        "priority": 2147483647,
        "track_width": 0.2,
        "via_diameter": 0.6,
        "via_drill": 0.3
      }
    ],
    "meta": {
      "version": 4
    }
  }
}
`, name+".kicad_pro")
}
