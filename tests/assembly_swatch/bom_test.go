// SPDX-License-Identifier: Apache-2.0

// Package bom_test reads each vendor's assembly swatch BOM: the rungs the
// vendor places, each with its LCSC number, MPN and manufacturer.
package bom_test

import (
	"encoding/csv"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	jlcpcb = flag.String("jlcpcb", "", "JLCPCB's BOM, as a runfiles path")
	pcbway = flag.String("pcbway", "", "PCBWay's BOM, as a runfiles path")
)

func rows(t *testing.T, rpath string) map[string]map[string]string {
	t.Helper()
	p, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	all, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, r := range all[1:] {
		m := map[string]string{}
		for i, h := range all[0] {
			m[h] = r[i]
		}
		if _, dup := out[m["LCSC"]]; dup || m["LCSC"] == "" {
			t.Errorf("%s: a row with no LCSC number or the same one twice: %v", rpath, m)
		}
		out[m["LCSC"]] = m
	}
	return out
}

// The parts of the ladder, by LCSC number.
var ladder = map[string]string{
	"C270365": "0201WMF1001TEE",
	"C11702":  "0402WGF1001TCE",
	"C21190":  "0603WAF1001T5E",
	"C17513":  "0805W8F1001T5E",
	"C4410":   "1206W4F1001T5E",
	"C2500":   "BAV99,215",
	"C7426":   "NE5532DR",
	"C5948":   "74HC595PW,118",
	"C730243": "74HC595BQ,115",
	"C23922":  "STM32F030C8T6",
}

func check(t *testing.T, name string, got map[string]map[string]string, want []string) {
	t.Helper()
	for _, lcsc := range want {
		r, ok := got[lcsc]
		if !ok {
			t.Errorf("%s: no %s (%s) in the BOM", name, lcsc, ladder[lcsc])
			continue
		}
		if r["MPN"] != ladder[lcsc] || r["Manufacturer"] == "" || r["Qty"] != "1" {
			t.Errorf("%s: %s: %v", name, lcsc, r)
		}
	}
	// Test points are bare pads on the board, not ordered parts.
	for _, r := range got {
		if strings.HasPrefix(r["Refs"], "TP") {
			t.Errorf("%s: a test point in the BOM: %v", name, r)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%s: %d BOM rows, want %d parts", name, len(got), len(want))
	}
}

func TestJLCPCB(t *testing.T) {
	// Economic PCBA: from 0402, so no 0201.
	got := rows(t, *jlcpcb)
	check(t, "jlcpcb", got, []string{"C11702", "C21190", "C17513", "C4410", "C2500", "C7426", "C5948", "C730243", "C23922"})
	if _, ok := got["C270365"]; ok {
		t.Error("jlcpcb: the 0201 resistor is in the BOM")
	}
}

func TestPCBWay(t *testing.T) {
	check(t, "pcbway", rows(t, *pcbway), []string{"C270365", "C11702", "C21190", "C17513", "C4410", "C2500", "C7426", "C5948", "C730243", "C23922"})
}
