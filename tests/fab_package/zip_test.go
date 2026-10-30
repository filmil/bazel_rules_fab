// SPDX-License-Identifier: Apache-2.0

// Package zip_test reads the ZIPs fab_package writes for the fixture board.
package zip_test

import (
	"archive/zip"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	zipPath = flag.String("zip", "", "the JLCPCB package, as a runfiles path")
	rotated = flag.String("rotated", "", "the same with a rotation correction")
)

func entries(t *testing.T, rpath string) map[string]string {
	t.Helper()
	p, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	out := map[string]string{}
	var names []string
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = string(b)
		names = append(names, f.Name)
	}
	out[""] = strings.Join(names, " ")
	return out
}

func TestJLCPCB(t *testing.T) {
	e := entries(t, *zipPath)
	want := "main-B_Cu.gbl main-B_Mask.gbs main-B_Paste.gbp main-B_Silkscreen.gbo " +
		"main-Edge_Cuts.gm1 main-F_Cu.gtl main-F_Mask.gts main-F_Paste.gtp " +
		"main-F_Silkscreen.gto main.drl main_bom.csv main_cpl.csv"
	if e[""] != want {
		t.Errorf("entries\n %s\nwant\n %s", e[""], want)
	}
	if bom := "Comment,Designator,Footprint,LCSC Part #\n" +
		"LED,D1,LED_0603_1608Metric,C2286\n" +
		"Power,J1,PinHeader_1x02_P2.54mm_Vertical,\n" +
		"1k,R1,R_0603_1608Metric,C21190\n"; e["main_bom.csv"] != bom {
		t.Errorf("main_bom.csv:\n%s\nwant:\n%s", e["main_bom.csv"], bom)
	}
	if cpl := "Designator,Mid X,Mid Y,Layer,Rotation\n" +
		"D1,115.0000mm,-102.5400mm,Top,0\n" +
		"J1,105.0000mm,-100.0000mm,Top,0\n" +
		"R1,115.0000mm,-100.0000mm,Bottom,90\n"; e["main_cpl.csv"] != cpl {
		t.Errorf("main_cpl.csv:\n%s\nwant:\n%s", e["main_cpl.csv"], cpl)
	}
}

func TestRotation(t *testing.T) {
	cpl := entries(t, *rotated)["main_cpl.csv"]
	if !strings.Contains(cpl, "D1,115.0000mm,-102.5400mm,Top,180\n") {
		t.Errorf("D1 is not turned:\n%s", cpl)
	}
	if !strings.Contains(cpl, "R1,115.0000mm,-100.0000mm,Bottom,90\n") {
		t.Errorf("R1 is turned:\n%s", cpl)
	}
}
