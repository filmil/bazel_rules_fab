// SPDX-License-Identifier: Apache-2.0

package assembly

import (
	"fmt"
	"strings"
	"testing"
)

// A small library: a two-pin part, derived ones, a two-unit part with a
// power unit, parts with stacked pins, malformed parts, and the test
// point and flag the schematic uses.
const lib = `(kicad_symbol_lib
	(version 20241209)
	(symbol "R"
		(property "Reference" "R")
		(property "Value" "R")
		(symbol "R_0_1"
			(rectangle (start -1 -2) (end 1 2))
		)
		(symbol "R_1_1"
			(pin passive line (at 0 3.81 270) (length 1.27) (name "~") (number "1"))
			(pin passive line (at 0 -3.81 90) (length 1.27) (name "~") (number "2"))
		)
	)
	(symbol "OPA"
		(property "Reference" "U")
		(property "Value" "OPA")
		(property "Description" "parent")
		(in_bom yes)
		(symbol "OPA_1_1"
			(pin input line (at -7.62 2.54 0) (length 2.54) (name "+") (number "3"))
			(pin input line (at -7.62 -2.54 0) (length 2.54) (name "-") (number "2"))
		)
		(symbol "OPA_2_1"
			(pin power_in line (at 0 7.62 270) (length 2.54) (name "V+") (number "8"))
			(pin power_in line (at 0 -7.62 90) (length 2.54) (name "V-") (number "4"))
		)
	)
	(symbol "CHILD"
		(extends "OPA")
		(property "Value" "CHILD")
		(property "Description" "a \"derived\" part")
	)
	(symbol "DNP"
		(extends "OPA")
		(in_bom no)
	)
	(symbol "BADCHILD"
		(extends "OPA")
		(symbol "BADCHILD_1_1")
	)
	(symbol "NOPIN"
		(symbol "NOPIN_1_1"
			(pin passive line (at 0 0 90) (length 0) (name "~"))
		)
	)
	(symbol "MCU"
		(symbol "MCU_1_1"
			(pin passive line (at 0 2.54 270) (length 2.54) (name "PA0") (number "1"))
			(pin passive line (at 0 -2.54 90) (length 2.54) (name "PB0") (number "2"))
			(pin power_in line (at 0 -2.54 90) (length 2.54) (hide yes) (name "VSS") (number "3"))
			(pin passive line (at 2.54 0 180) (length 2.54) (name "PA1") (number "4"))
		)
	)
	(symbol "AMP"
		(symbol "AMP_1_1"
			(pin input line (at -5.08 2.54 0) (length 2.54) (name "IN") (number "1"))
			(pin power_in line (at 0 5.08 270) (length 2.54) (name "V+") (number "2"))
			(pin power_in line (at 0 -5.08 90) (length 2.54) (name "V-") (number "3"))
			(pin passive line (at 0 -5.08 90) (length 2.54) (hide yes) (name "EP") (number "4"))
		)
	)
	(symbol "SHORT"
		(symbol "SHORT_1_1"
			(pin passive line (at 0 0 90) (length 2.54) (name "A") (number "1"))
			(pin passive line (at 0 0 90) (length 2.54) (name "B") (number "2"))
		)
	)
	(symbol "TestPoint"
		(property "Reference" "TP")
		(symbol "TestPoint_1_1"
			(pin passive line (at 0 0 90) (length 0) (name "1") (number "1"))
		)
	)
	(symbol "PWR_FLAG"
		(power)
		(property "Reference" "#FLG")
		(symbol "PWR_FLAG_0_0"
			(pin power_out line (at 0 0 90) (length 0) (name "~") (number "1"))
		)
	)
)`

func libs(t *testing.T) Libraries {
	t.Helper()
	l, err := ReadLibrary("Lib", lib)
	if err != nil {
		t.Fatal(err)
	}
	return Libraries{"Lib": l, "Connector": rename(t, "Connector"), "power": rename(t, "power")}
}

func rename(t *testing.T, name string) *Library {
	l, err := ReadLibrary(name, lib)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestExtends(t *testing.T) {
	s, err := libs(t)["Lib"].Symbol("CHILD")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	Write(&b, s, 0)
	out := b.String()
	for _, want := range []string{`(symbol "Lib:CHILD"`, `(symbol "CHILD_1_1"`, `(symbol "CHILD_2_1"`, `(property "Value" "CHILD"`, `"a \"derived\" part"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the derived symbol lacks %s:\n%s", want, out)
		}
	}
	for _, bad := range []string{"extends", "OPA_", `"parent"`} {
		if strings.Contains(out, bad) {
			t.Errorf("the derived symbol keeps %s:\n%s", bad, out)
		}
	}
	units, err := Units(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || len(units[1]) != 2 || units[2][0].Type != "power_in" {
		t.Errorf("units %+v", units)
	}
}

func TestExtendsAttributes(t *testing.T) {
	s, err := libs(t)["Lib"].Symbol("DNP")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	Write(&b, s, 0)
	if out := b.String(); strings.Count(out, "(in_bom") != 1 || !strings.Contains(out, "(in_bom no)") {
		t.Errorf("the derived symbol's in_bom does not replace its parent's:\n%s", out)
	}
	if _, err := libs(t)["Lib"].Symbol("BADCHILD"); err == nil || !strings.Contains(err.Error(), "units of its own") {
		t.Errorf("a derived symbol with units: %v", err)
	}
	if _, err := libs(t)["Lib"].Symbol("NOPIN"); err != nil {
		t.Fatal(err)
	}
	nopin, _ := libs(t)["Lib"].Symbol("NOPIN")
	if _, err := Units(nopin); err == nil || !strings.Contains(err.Error(), "lacks at, number or name") {
		t.Errorf("a pin with no number: %v", err)
	}
}

// A measured pin stacked with a hidden power pin: one label, no
// no-connect flag on the stack, and a PWR_FLAG on its net.
func TestStackedPins(t *testing.T) {
	out, err := Schematic(spec(Rung{Ref: "U1", Symbol: "Lib:MCU", Footprint: "F:U", Value: "MCU", Measure: []string{"1", "2"}}), libs(t))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, `(label "U1_B"`); n != 3 {
		t.Errorf("%d U1_B labels, want 3 (pin stack, test point, flag)", n)
	}
	if !strings.Contains(out, `(reference "#FLG01")`) {
		t.Error("the stacked power pin's net has no PWR_FLAG")
	}
	if n := strings.Count(out, "(no_connect"); n != 1 {
		t.Errorf("%d no-connect flags, want 1 (PA1)", n)
	}
	if _, err := Schematic(spec(Rung{Ref: "U1", Symbol: "Lib:SHORT", Measure: []string{"1", "2"}}), libs(t)); err == nil || !strings.Contains(err.Error(), "stacked") {
		t.Errorf("two measured pins stacked: %v", err)
	}
}

func spec(rungs ...Rung) Spec { return Spec{Project: "p", Title: "t", Rungs: rungs} }

func TestSchematic(t *testing.T) {
	out, err := Schematic(spec(
		Rung{Ref: "R1", Symbol: "Lib:R", Footprint: "F:R", Value: "1k", Measure: []string{"1", "2"}, Fields: map[string]string{"LCSC": "C1"}},
		Rung{Ref: "U1", Symbol: "Lib:CHILD", Footprint: "F:U", Value: "CHILD", Measure: []string{"3", "2"}},
	), libs(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`(lib_id "Lib:CHILD")`,
		`(label "R1_A"`, `(label "R1_B"`, `(label "U1_A"`,
		`(property "LCSC" "C1"`,
		`(lib_id "Connector:TestPoint")`,
		`(reference "TP4")`,
		`(in_bom no)` + "\n\t\t(on_board yes)", // test points: on the board, not in the BOM
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the schematic lacks %q", want)
		}
	}
	// U1's power unit is placed, its pins flagged; no PWR_FLAG, as no
	// measured net holds a power pin.
	if !strings.Contains(out, "(unit 2)") {
		t.Error("U1's power unit is not placed")
	}
	if strings.Contains(out, `(reference "#FLG`) {
		t.Error("a PWR_FLAG, though no measured net holds a power pin")
	}
	if n := strings.Count(out, "(no_connect"); n != 2 {
		t.Errorf("%d no-connect flags, want 2 (V+, V-)", n)
	}
	if strings.Count(out, "(") != strings.Count(out, ")") {
		t.Error("unbalanced parentheses")
	}
	again, _ := Schematic(spec(
		Rung{Ref: "R1", Symbol: "Lib:R", Footprint: "F:R", Value: "1k", Measure: []string{"1", "2"}, Fields: map[string]string{"LCSC": "C1"}},
		Rung{Ref: "U1", Symbol: "Lib:CHILD", Footprint: "F:U", Value: "CHILD", Measure: []string{"3", "2"}},
	), libs(t))
	if again != out {
		t.Error("two schematics of one spec differ")
	}
}

func TestPowerFlag(t *testing.T) {
	out, err := Schematic(spec(Rung{Ref: "U1", Symbol: "Lib:OPA", Footprint: "F:U", Value: "OPA", Measure: []string{"4", "3"}}), libs(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `(reference "#FLG01")`) || strings.Count(out, `(label "U1_A"`) != 3 {
		t.Errorf("a measured power pin's net has no PWR_FLAG, or not three U1_A labels (pin, test point, flag):\n%s", out)
	}
}

func TestSchematicErrors(t *testing.T) {
	r := Rung{Ref: "R1", Symbol: "Lib:R", Measure: []string{"1", "2"}}
	for _, c := range []struct {
		spec Spec
		want string
	}{
		{Spec{}, "needs a project and rungs"},
		{spec(r, r), "R1 appears twice"},
		{spec(Rung{Ref: "R1", Symbol: "Lib:R", Measure: []string{"1"}}), "two different pins"},
		{spec(Rung{Ref: "R1", Symbol: "Lib:R", Measure: []string{"1", "1"}}), "two different pins"},
		{spec(Rung{Ref: "R1", Symbol: "Lib:R", Measure: []string{"1", "9"}}), `has no pin "9"`},
		{spec(Rung{Ref: "R1", Symbol: "Lib:NOPE", Measure: []string{"1", "2"}}), `no symbol "NOPE"`},
		{spec(Rung{Ref: "R1", Symbol: "Other:R", Measure: []string{"1", "2"}}), "no library Other"},
		{spec(Rung{Ref: "R1", Symbol: "R", Measure: []string{"1", "2"}}), "want Lib:Name"},
	} {
		if _, err := Schematic(c.spec, libs(t)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want %q", c.spec, err, c.want)
		}
	}
	if _, err := ReadLibrary("x", "(not_a_lib)"); err == nil {
		t.Error("a non-library read")
	}
}

// Footprints for the board: a two-pad part with silkscreen clear of its
// pads, a four-pad part whose pads 2 and 3 share a net (the MCU's stacked
// pins), and KiCad's test point, whose silkscreen circle comes 0.14 mm
// from its pad.
const (
	fpR = `(footprint "R"
	(layer "F.Cu")
	(property "Reference" "REF**" (at 0 -1.2 0) (layer "F.SilkS") (uuid "u0"))
	(property "Value" "R" (at 0 1.2 0) (layer "F.Fab") (uuid "u1"))
	(property "Datasheet" "" (at 0 0 0) (layer "F.Fab") (hide yes) (uuid "u2"))
	(attr smd)
	(fp_line (start -0.3 -0.55) (end 0.3 -0.55) (stroke (width 0.12) (type solid)) (layer "F.SilkS") (uuid "u3"))
	(fp_rect (start -1.6 -0.6) (end 1.6 0.6) (stroke (width 0.05) (type solid)) (fill no) (layer "F.CrtYd") (uuid "u4"))
	(pad "1" smd rect (at -1 0) (size 0.8 0.8) (layers "F.Cu" "F.Mask" "F.Paste") (uuid "u5"))
	(pad "2" smd rect (at 1 0) (size 0.8 0.8) (layers "F.Cu" "F.Mask" "F.Paste") (uuid "u6"))
	(model "${KICAD9_3DMODEL_DIR}/F.3dshapes/R.step")
)`
	fpU = `(footprint "U"
	(layer "F.Cu")
	(property "Reference" "REF**" (at 0 -3 0) (layer "F.SilkS") (uuid "v0"))
	(property "Value" "U" (at 0 3 0) (layer "F.Fab") (uuid "v1"))
	(attr smd)
	(fp_rect (start -2 -2) (end 2 2) (stroke (width 0.05) (type solid)) (fill no) (layer "F.CrtYd") (uuid "v2"))
	(pad "1" smd roundrect (at -1.5 0) (size 0.6 0.3) (layers "F.Cu" "F.Mask") (roundrect_rratio 0.25) (uuid "v3"))
	(pad "2" smd roundrect (at 0 1.5) (size 0.3 0.6) (layers "F.Cu" "F.Mask") (roundrect_rratio 0.25) (uuid "v4"))
	(pad "3" smd roundrect (at 0 -1.5) (size 0.3 0.6) (layers "F.Cu" "F.Mask") (roundrect_rratio 0.25) (uuid "v5"))
	(pad "4" smd roundrect (at 1.5 0) (size 0.6 0.3) (layers "F.Cu" "F.Mask") (roundrect_rratio 0.25) (uuid "v6"))
)`
	fpTP = `(footprint "TestPoint_Pad_D1.5mm"
	(version 20241229)
	(generator "pcbnew")
	(layer "F.Cu")
	(property "Reference" "REF**" (at 0 -1.648 0) (layer "F.SilkS") (uuid "w0"))
	(property "Value" "TestPoint_Pad_D1.5mm" (at 0 1.75 0) (layer "F.Fab") (uuid "w1"))
	(attr exclude_from_pos_files exclude_from_bom)
	(fp_circle (center 0 0) (end 0 0.95) (stroke (width 0.12) (type solid)) (fill no) (layer "F.SilkS") (uuid "w2"))
	(fp_circle (center 0 0) (end 1.25 0) (stroke (width 0.05) (type solid)) (fill no) (layer "F.CrtYd") (uuid "w3"))
	(pad "1" smd circle (at 0 0) (size 1.5 1.5) (layers "F.Cu" "F.Mask") (uuid "w4"))
)`
)

func footprints(t *testing.T) Footprints {
	t.Helper()
	fps := Footprints{}
	for id, text := range map[string]string{"F:R": fpR, "F:U": fpU, testPointID: fpTP} {
		fp, err := ReadFootprint(text)
		if err != nil {
			t.Fatal(err)
		}
		fps[id] = fp
	}
	return fps
}

func boardSpec() Spec {
	return spec(
		Rung{Ref: "R1", Symbol: "Lib:R", Footprint: "F:R", Value: "1k", Label: "0402", Silk: "1.00k 1%", Measure: []string{"1", "2"}, Fields: map[string]string{"LCSC": "C1", "Datasheet": "https://example.com/r.pdf"}},
		Rung{Ref: "U1", Symbol: "Lib:MCU", Footprint: "F:U", Value: "MCU", Label: "QFP", Silk: "0.6V", Measure: []string{"1", "2"}},
	)
}

func TestDesign(t *testing.T) {
	sch, pcb, pairs, err := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	// Each footprint names its symbol's UUID, from the schematic.
	for _, ref := range []string{"R1", "U1", "TP1", "TP4"} {
		i := strings.Index(sch, `(reference "`+ref+`")`)
		if i < 0 {
			t.Fatalf("no %s in the schematic", ref)
		}
	}
	paths := strings.Count(pcb, "(path \"/")
	if paths != 6 {
		t.Errorf("%d footprints with a symbol path, want 6 (R1, U1, four test points)", paths)
	}
	for _, want := range []string{
		`(footprint "F:R"`, `(footprint "F:U"`, `(footprint "TestPoint:TestPoint_Pad_D1.5mm"`,
		`(net 1 "/R1_A")`, `(property "LCSC" "C1"`, `(property "Datasheet" "https://example.com/r.pdf"`,
		`(property "Value" "R1_A"`, // a test point's value is its net's label
		`(gr_text "0402  1.00k 1%"`, `(gr_text "+"`, `(gr_text "-"`,
		`(model "${KICAD9_3DMODEL_DIR}/F.3dshapes/R.step"`, // the library's model, kept
	} {
		if !strings.Contains(pcb, want) {
			t.Errorf("the board lacks %s", want)
		}
	}
	for _, bad := range []string{"(version 20241229)\n\t\t", "(generator \"pcbnew\")", `"u5"`} {
		if strings.Contains(pcb, bad) {
			t.Errorf("the board keeps the library's %s", bad)
		}
	}
	// U1's pad 3 is stacked with pad 2 in the symbol: it shares U1_B's net
	// and joins its track, round the part (more than the two straight
	// tracks of R1, and U1's own two).
	if !strings.Contains(pcb, `(net 4 "/U1_B")`) {
		t.Errorf("the nets: %s", pcb[:strings.Index(pcb, "(gr_line")])
	}
	if n := strings.Count(pcb, "(segment"); n <= 8 {
		t.Errorf("%d track segments; want more than 8 (pad, ring, test point for four pads, and pad 3's way round)", n)
	}
	// Each of the four test points' silkscreen circle comes 0.14 mm from
	// its pad; R1's silkscreen line is clear.
	if pairs != 4 {
		t.Errorf("%d own-silkscreen pairs, want 4", pairs)
	}
	_, again, pairs2, _ := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15})
	if again != pcb || pairs2 != pairs {
		t.Error("two boards of one spec differ")
	}
	if strings.Count(pcb, "(") != strings.Count(pcb, ")") {
		t.Error("unbalanced parentheses")
	}
	if _, err := Parse(pcb); err != nil {
		t.Errorf("the board does not parse: %v", err)
	}
	_, _, pairs3, _ := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.13})
	if pairs3 != 0 {
		t.Errorf("%d pairs under a 0.13 mm clearance, want 0", pairs3)
	}
}

func TestBads(t *testing.T) {
	_, good, _, _ := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15})
	_, label, _, err := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15, Bad: "label_on_pad"})
	if err != nil {
		t.Fatal(err)
	}
	if label == good {
		t.Error("label_on_pad changed nothing")
	}
	_, near, _, err := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15, Bad: "silk_near_pad"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(near, `(layer "F.SilkS")`+"\n\t\t(uuid") <= strings.Count(good, `(layer "F.SilkS")`+"\n\t\t(uuid") {
		t.Error("silk_near_pad drew no silkscreen line")
	}
	if _, _, _, err := Design(boardSpec(), libs(t), footprints(t), BoardOptions{SilkClear: 0.15, Bad: "other"}); err == nil {
		t.Error("an unknown bad")
	}
}

func TestBoardErrors(t *testing.T) {
	fps := footprints(t)
	for _, c := range []struct {
		name string
		fps  func() Footprints
		spec Spec
		o    BoardOptions
		want string
	}{
		{"no clearance", nil, boardSpec(), BoardOptions{}, "no pad-to-silkscreen clearance"},
		{"no test point", func() Footprints { f := footprints(t); delete(f, testPointID); return f }, boardSpec(), BoardOptions{SilkClear: 0.15}, "no footprint TestPoint"},
		{"no part footprint", func() Footprints { f := footprints(t); delete(f, "F:U"); return f }, boardSpec(), BoardOptions{SilkClear: 0.15}, "U1: no footprint F:U"},
		{"no pad", nil, spec(Rung{Ref: "R1", Symbol: "Lib:MCU", Footprint: "F:R", Measure: []string{"1", "4"}}), BoardOptions{SilkClear: 0.15}, `has no pad "4"`},
	} {
		f := fps
		if c.fps != nil {
			f = c.fps()
		}
		if _, _, _, err := Design(c.spec, libs(t), f, c.o); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	// Too many rungs for the card: about 45 of these fit.
	var rungs []Rung
	for i := 1; i <= 80; i++ {
		rungs = append(rungs, Rung{Ref: fmt.Sprintf("U%d", i), Symbol: "Lib:MCU", Footprint: "F:U", Measure: []string{"1", "2"}})
	}
	if _, _, _, err := Design(spec(rungs...), libs(t), fps, BoardOptions{SilkClear: 0.15}); err == nil || !strings.Contains(err.Error(), "do not fit on the card") {
		t.Errorf("80 rungs: %v", err)
	}
}

func TestOwnSilkPairs(t *testing.T) {
	fp := func(silk string) *Node {
		n, err := ReadFootprint(`(footprint "X" (layer "F.Cu") ` + silk + ` (pad "1" smd roundrect (at 0 0) (size 1 0.5) (layers "F.Cu") (roundrect_rratio 0.2)))`)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	line := func(y float64) string {
		return fmt.Sprintf(`(fp_line (start -0.4 %g) (end 0.4 %g) (stroke (width 0.1)) (layer "F.SilkS"))`, y, y)
	}
	for _, c := range []struct {
		silk string
		want int
	}{
		// Above the pad's top edge (y = -0.25): the gap is the line's
		// distance, less half its width.
		{line(-0.25 - 0.10 - 0.05), 1}, // 0.10
		{line(-0.25 - 0.20 - 0.05), 0}, // 0.20
		{`(fp_line (start -1 -1) (end 1 -1) (stroke (width 0.1)) (layer "F.Fab"))`, 0},            // not silkscreen
		{`(fp_circle (center 0 0) (end 0.70 0) (stroke (width 0.1)) (layer "F.SilkS"))`, 1},       // 0.123 from the far corners
		{`(fp_rect (start -0.65 -0.4) (end 0.65 0.4) (stroke (width 0.1)) (layer "F.SilkS"))`, 1}, // 0.10 from the ends
		{`(fp_poly (pts (xy -0.2 -0.4) (xy 0.2 -0.4) (xy 0 -0.6)) (stroke (width 0.1)) (layer "F.SilkS"))`, 1},
		{`(fp_arc (start -0.6 -0.4) (mid 0 -0.5) (end 0.6 -0.4) (stroke (width 0.1)) (layer "F.SilkS"))`, 1},
	} {
		n, err := OwnSilkPairs("X1", fp(c.silk), 0.15)
		if err != nil || n != c.want {
			t.Errorf("%s: %d pairs, %v; want %d", c.silk, n, err, c.want)
		}
	}
	// A line on the pad: under even the extra rule's 0.09 mm.
	if _, err := OwnSilkPairs("X1", fp(line(0)), 0.15); err == nil || !strings.Contains(err.Error(), "under even 0.09 mm") {
		t.Errorf("silkscreen on the pad: %v", err)
	}
	// A rounded corner is farther than a square one: a line past the
	// corner, diagonally, clears the round rectangle.
	diag := `(fp_line (start 0.62 -0.37) (end 0.7 -0.45) (stroke (width 0.02)) (layer "F.SilkS"))`
	if n, _ := OwnSilkPairs("X1", fp(diag), 0.15); n != 0 {
		t.Errorf("a line past a rounded corner: %d pairs, want 0", n)
	}
}

func TestAround(t *testing.T) {
	cy := box{-1, -1, 1, 1}
	r := ringGap
	top, bottom, right := pt{0, -1 - r}, pt{0, 1 + r}, pt{1 + r, 0}
	// From the top to the bottom, the right side blocked: the left way.
	way, err := around(cy, top, bottom, []pt{right})
	if err != nil || len(way) != 2 || way[0].x >= 0 || way[1].x >= 0 {
		t.Errorf("around the left: %v, %v", way, err)
	}
	// Both ways blocked.
	if _, err := around(cy, top, bottom, []pt{right, {-1 - r, 0}}); err == nil {
		t.Error("both ways blocked, but a way found")
	}
	// Top to right, nothing blocked: the short way, by one corner.
	way, err = around(cy, top, right, nil)
	if err != nil || len(way) != 1 || way[0] != (pt{1 + r, -1 - r}) {
		t.Errorf("top to right: %v, %v", way, err)
	}
}

func TestExitsTooClose(t *testing.T) {
	fp, err := ReadFootprint(`(footprint "X" (layer "F.Cu")
		(fp_rect (start -1 -1) (end 1 1) (layer "F.CrtYd"))
		(pad "1" smd rect (at -0.8 -0.25) (size 0.3 0.2) (layers "F.Cu"))
		(pad "2" smd rect (at -0.8 0.25) (size 0.3 0.2) (layers "F.Cu"))
		(pad "3" smd rect (at 0.8 0) (size 0.3 0.2) (layers "F.Cu")))`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exits(Rung{Ref: "U1", Footprint: "X", Measure: []string{"1", "2"}}, fp); err == nil || !strings.Contains(err.Error(), "too close for two test points") {
		t.Errorf("two pads 0.5 mm apart on one side: %v", err)
	}
	ex, err := exits(Rung{Ref: "U1", Footprint: "X", Measure: []string{"1", "3"}}, fp)
	if err != nil || ex[0].dir != (pt{-1, 0}) || ex[1].dir != (pt{1, 0}) {
		t.Errorf("opposite sides: %+v, %v", ex, err)
	}
}

// An op-amp whose V- pin and exposed pad are stacked in its symbol, as
// KiCad's TLV9064xRTE: their net, which no test point measures, gets a
// track from the V- pad into the exposed pad.
func TestJoinCenter(t *testing.T) {
	ep := func(at string) string {
		return `(footprint "Q" (layer "F.Cu")
	(fp_rect (start -2 -2) (end 2 2) (stroke (width 0.05) (type solid)) (fill no) (layer "F.CrtYd"))
	(pad "1" smd rect (at -1.5 0) (size 0.6 0.25) (layers "F.Cu"))
	(pad "2" smd rect (at 0 -1.5) (size 0.25 0.6) (layers "F.Cu"))
	(pad "3" smd rect (at 1.5 0.25) (size 0.6 0.25) (layers "F.Cu"))
	(pad "4" smd rect (at ` + at + `) (size 1.7 1.7) (layers "F.Cu")))`
	}
	fps := footprints(t)
	amp := spec(Rung{Ref: "U1", Symbol: "Lib:AMP", Footprint: "F:Q", Value: "AMP", Measure: []string{"1", "2"}})
	var err error
	if fps["F:Q"], err = ReadFootprint(ep("0 0")); err != nil {
		t.Fatal(err)
	}
	_, pcb, _, err := Design(amp, libs(t), fps, BoardOptions{SilkClear: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	net := "unconnected-(U1-V--Pad3)"
	i := strings.Index(pcb, fmt.Sprintf("%q)", net))
	if i < 0 {
		t.Fatalf("no net %s:\n%s", net, pcb[:strings.Index(pcb, "(gr_line")])
	}
	var id int
	fmt.Sscanf(pcb[strings.LastIndex(pcb[:i], "(net ")+5:], "%d", &id)
	// From the V- pad at (1.5, 0.25) to the pad's middle line, at the
	// card origin's offset.
	if strings.Count(pcb, fmt.Sprintf("(net %d)\n", id)) != 1 {
		t.Errorf("want one track on %s (id %d)", net, id)
	}
	// The exposed pad off the middle: the pads cannot be joined.
	if fps["F:Q"], err = ReadFootprint(ep("1.6 1.6")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Design(amp, libs(t), fps, BoardOptions{SilkClear: 0.15}); err == nil || !strings.Contains(err.Error(), "only an edge pad and the center pad") {
		t.Errorf("pads off the middle: %v", err)
	}
}

// Test points that leave on perpendicular sides: each polarity mark sits
// on its own test point's far side from the other test point, measured
// along its own lateral axis.
func TestLabelsPerpendicular(t *testing.T) {
	fp := footprints(t)["F:U"]
	// Pad 1 leaves left, pad 2 down: the left test point's mark goes up,
	// away from the lower test point.
	c, err := layout(Rung{Ref: "U1", Footprint: "F:U", Measure: []string{"1", "2"}}, fp)
	if err != nil {
		t.Fatal(err)
	}
	left, down := c.exits[0].tp(), c.exits[1].tp()
	if c.labels[0].y >= left.y {
		t.Errorf("the left mark at y %v, its test point at %v: want it above, away from the lower one at %v", c.labels[0].y, left.y, down.y)
	}
	if (c.labels[1].x > down.x) != (left.x < down.x) {
		t.Errorf("the lower mark at x %v, its test point at %v, the left one at %v", c.labels[1].x, down.x, left.x)
	}
	// The cell holds the ring round the courtyard.
	cy := courtyard(fp)
	if c.bounds.x1 < cy.x1+ringGap+trackWidth/2 || c.bounds.y0 > cy.y0-ringGap-trackWidth/2 {
		t.Errorf("the cell %+v does not hold the ring round %+v", c.bounds, cy)
	}
}

func TestOwnSilkFilled(t *testing.T) {
	fp, err := ReadFootprint(`(footprint "X" (layer "F.Cu")
		(fp_poly (pts (xy -1 -1) (xy 1 -1) (xy 1 1) (xy -1 1)) (stroke (width 0)) (fill solid) (layer "F.SilkS"))
		(pad "1" smd rect (at 0 0) (size 0.4 0.4) (layers "F.Cu")))`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OwnSilkPairs("X1", fp, 0.15); err == nil || !strings.Contains(err.Error(), "under even") {
		t.Errorf("a filled square over the pad: %v", err)
	}
}

func TestJoinCenterClearance(t *testing.T) {
	fps := footprints(t)
	var err error
	// Pad 5, in no symbol, sits by the track from pad 3 into the center.
	fps["F:Q"], err = ReadFootprint(`(footprint "Q" (layer "F.Cu")
	(fp_rect (start -2 -2) (end 2 2) (stroke (width 0.05) (type solid)) (fill no) (layer "F.CrtYd"))
	(pad "1" smd rect (at -1.5 0) (size 0.6 0.25) (layers "F.Cu"))
	(pad "2" smd rect (at 0 -1.5) (size 0.25 0.6) (layers "F.Cu"))
	(pad "3" smd rect (at 1.5 0.25) (size 0.6 0.25) (layers "F.Cu"))
	(pad "4" smd rect (at 0 0) (size 1.0 1.0) (layers "F.Cu"))
	(pad "5" smd rect (at 0.9 0.5) (size 0.2 0.2) (layers "F.Cu")))`)
	if err != nil {
		t.Fatal(err)
	}
	amp := spec(Rung{Ref: "U1", Symbol: "Lib:AMP", Footprint: "F:Q", Value: "AMP", Measure: []string{"1", "2"}})
	if _, _, _, err := Design(amp, libs(t), fps, BoardOptions{SilkClear: 0.15}); err == nil || !strings.Contains(err.Error(), "from pad 5") {
		t.Errorf("a track by pad 5: %v", err)
	}
}
