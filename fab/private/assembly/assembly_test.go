// SPDX-License-Identifier: Apache-2.0

package assembly

import (
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
