// SPDX-License-Identifier: Apache-2.0

package pkgcheck

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, d := range files {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(d))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func good() map[string]string {
	f := map[string]string{
		"main_bom.csv": "Comment,Designator,Footprint,LCSC Part #\nLED,D1,LED_0603,C2286\n",
		"main_cpl.csv": "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1.0000mm,-2.5000mm,Top,90\n",
		"main.drl":     "M48",
	}
	for _, l := range []string{"F_Cu.gtl", "B_Cu.gbl", "F_Mask.gts", "B_Mask.gbs",
		"F_Silkscreen.gto", "B_Silkscreen.gbo", "Edge_Cuts.gm1", "F_Paste.gtp", "In1_Cu.g1"} {
		f["main-"+l] = "G04*"
	}
	return f
}

func check(t *testing.T, vendor string, f map[string]string) string {
	t.Helper()
	p, err := Check(vendor, zipOf(t, f))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(p, "\n")
}

func TestGood(t *testing.T) {
	if p := check(t, "jlcpcb", good()); p != "" {
		t.Errorf("problems in a good ZIP:\n%s", p)
	}
	f := good()
	delete(f, "main_cpl.csv")
	f["main_bom.csv"] = "Item #,Designator,Qty,Manufacturer,Mfg Part #,Description / Value,Package/Footprint,Type,Your Instructions / Notes\n1,D1,1,,,LED,LED_0603,,\n"
	f["main_centroid.csv"] = "Designator,Footprint,Mid X,Mid Y,TB,Rotation,Comment\nD1,LED_0603,1.0000mm,2.0000mm,B,0,LED\n"
	if p := check(t, "pcbway", f); p != "" {
		t.Errorf("problems in a good PCBWay ZIP:\n%s", p)
	}
}

func TestBad(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(map[string]string)
		want   string
	}{
		{"missing layer", func(f map[string]string) { delete(f, "main-B_Cu.gbl") }, "no bottom copper (B_Cu) (*.gbl)"},
		{"missing drill", func(f map[string]string) { delete(f, "main.drl") }, "no drill file (*.drl)"},
		{"missing BOM", func(f map[string]string) { delete(f, "main_bom.csv") }, "no BOM (*_bom.csv)"},
		{"missing CPL", func(f map[string]string) { delete(f, "main_cpl.csv") }, "no placement list (*_cpl.csv)"},
		{"wrong column", func(f map[string]string) {
			f["main_bom.csv"] = "Value,Designator,Footprint,LCSC Part #\nLED,D1,LED_0603,\n"
		}, `main_bom.csv: columns ["Value" "Designator" "Footprint" "LCSC Part #"]`},
		{"no rows", func(f map[string]string) { f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\n" }, "main_cpl.csv: no rows"},
		{"inches", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,40mil,1.0000mm,Top,0\n"
		}, `D1: Mid X "40mil" is not in mm`},
		{"rotation", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1mm,1mm,Top,360\n"
		}, `Rotation "360" is not at least 0 and below 360`},
		{"layer", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1mm,1mm,top,0\n"
		}, `Layer "top" is not one of Top, Bottom`},
		{"no designator", func(f map[string]string) {
			f["main_bom.csv"] = "Comment,Designator,Footprint,LCSC Part #\nLED,,LED_0603,\n"
		}, "main_bom.csv: a row with no Designator"},
		{"extra file", func(f map[string]string) { f["main-job.gbrjob"] = "{}" }, "main-job.gbrjob: not a file jlcpcb takes"},
		{"subdirectory", func(f map[string]string) { f["gerbers/main-F_Cu.gtl"] = "G04*" }, "gerbers/main-F_Cu.gtl: not at the ZIP's root"},
		{"empty layer", func(f map[string]string) { f["main-F_Cu.gtl"] = "" }, "main-F_Cu.gtl: empty"},
		{"two boards", func(f map[string]string) {
			f["other_cpl.csv"] = f["main_cpl.csv"]
			delete(f, "main_cpl.csv")
		}, "name different boards"},
		{"two BOMs", func(f map[string]string) { f["b_bom.csv"] = f["main_bom.csv"] }, "two BOMs: b_bom.csv, main_bom.csv"},
		{"two CPLs", func(f map[string]string) { f["b_cpl.csv"] = f["main_cpl.csv"] }, "two placement lists"},
		{"bad second BOM", func(f map[string]string) { f["b_bom.csv"] = "x\n" }, "b_bom.csv: columns"},
		{"two top copper", func(f map[string]string) { f["other-F_Cu.gtl"] = "G04*" }, "two files of top copper (F_Cu)"},
		{"no board name", func(f map[string]string) { f["_bom.csv"] = f["main_bom.csv"] }, "_bom.csv: no board name"},
		{"inner 31", func(f map[string]string) { f["main-In31_Cu.g31"] = "G04*" }, "main-In31_Cu.g31: not a file"},
		{"CPL no designator", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\n,1mm,1mm,Top,0\n"
		}, "main_cpl.csv: a row with no Designator"},
		{"NaN rotation", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1mm,1mm,Top,NaN\n"
		}, `Rotation "NaN" is not`},
		{"negative rotation", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1mm,1mm,Top,-90\n"
		}, `Rotation "-90" is not`},
		{"word rotation", func(f map[string]string) {
			f["main_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nD1,1mm,1mm,Top,left\n"
		}, `Rotation "left" is not`},
		{"malformed CSV", func(f map[string]string) { f["main_bom.csv"] = "a,b\n\"x\n" }, "main_bom.csv: parse error"},
		{"empty CSV", func(f map[string]string) { f["main_cpl.csv"] = " " }, "main_cpl.csv:"},
	} {
		f := good()
		c.change(f)
		if p := check(t, "jlcpcb", f); !strings.Contains(p, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.want, p)
		}
	}
}

func TestWrongVendor(t *testing.T) {
	p := check(t, "pcbway", good())
	for _, want := range []string{"no placement list (*_centroid.csv)", "main_bom.csv: columns", "main_cpl.csv: not a file pcbway takes"} {
		if !strings.Contains(p, want) {
			t.Errorf("want %q in:\n%s", want, p)
		}
	}
}

func TestVerdict(t *testing.T) {
	for _, c := range []struct {
		problems []string
		want     string
		ok       bool
	}{
		{nil, "", true},
		{[]string{"no BOM"}, "", false},
		{[]string{"no BOM", "x"}, "no BOM", true},
		{[]string{"x"}, "no BOM", false},
		{nil, "no BOM", false}, // a known-bad test whose ZIP is good fails
	} {
		if err := Verdict(c.problems, c.want); (err == nil) != c.ok {
			t.Errorf("Verdict(%q, %q) = %v", c.problems, c.want, err)
		}
	}
}

func TestErrors(t *testing.T) {
	if _, err := Check("oshpark", nil); err == nil || !strings.Contains(err.Error(), "want one of jlcpcb, pcbway") {
		t.Errorf("unknown vendor: %v", err)
	}
	if _, err := Check("jlcpcb", []byte("not a zip")); err == nil || !strings.Contains(err.Error(), "not a ZIP") {
		t.Errorf("not a zip: %v", err)
	}
}
