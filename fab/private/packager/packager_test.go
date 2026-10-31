// SPDX-License-Identifier: Apache-2.0

package packager

import (
	"archive/zip"
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

const bomIn = `"Refs","Value","Footprint","Qty","DNP","LCSC"
"D1","LED","LED_SMD:LED_0603_1608Metric","1","","C2286"
"J1","Power","Connector:PinHeader_1x02","1","",""
"R1,R2","1k","Resistor_SMD:R_0603_1608Metric","2","",""
"R3","10k","Resistor_SMD:R_0603_1608Metric","1","DNP",""
`

const posIn = `Ref,Val,Package,PosX,PosY,Rot,Side
"D1","LED","LED_0603_1608Metric",115.000000,-102.540000,0.000000,top
"J1","Power","PinHeader_1x02",105.000000,-100.000000,270.000000,top
"R1","1k","R_0603_1608Metric",115.000000,-100.000000,90.000000,bottom
`

var jlc = Layouts["jlcpcb"]

func TestBOM(t *testing.T) {
	got, err := BOM(jlc, []byte(bomIn))
	if err != nil {
		t.Fatal(err)
	}
	want := "Comment,Designator,Footprint,LCSC Part #\n" +
		"LED,D1,LED_0603_1608Metric,C2286\n" +
		"Power,J1,PinHeader_1x02,\n" +
		"1k,\"R1,R2\",R_0603_1608Metric,\n"
	if string(got) != want {
		t.Errorf("BOM:\n%s\nwant:\n%s", got, want)
	}
}

func TestBOMWithoutOptionalColumns(t *testing.T) {
	got, err := BOM(jlc, []byte("Refs,Value,Footprint\nC1,100n,C_0402\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Comment,Designator,Footprint,LCSC Part #\n100n,C1,C_0402,\n"; string(got) != want {
		t.Errorf("BOM %q, want %q", got, want)
	}
}

func TestCPL(t *testing.T) {
	r, err := ParseRotation("LED_.*=180")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ParseRotation("R_0603.*=-90")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CPL(jlc, []byte(posIn), []Rotation{r, r2})
	if err != nil {
		t.Fatal(err)
	}
	want := "Designator,Mid X,Mid Y,Layer,Rotation\n" +
		"D1,115.0000mm,-102.5400mm,Top,180\n" +
		"J1,105.0000mm,-100.0000mm,Top,270\n" +
		"R1,115.0000mm,-100.0000mm,Bottom,0\n"
	if string(got) != want {
		t.Errorf("CPL:\n%s\nwant:\n%s", got, want)
	}
}

func TestFirstRotationWins(t *testing.T) {
	specific, _ := ParseRotation("LED_0603_1608Metric=90")
	broad, _ := ParseRotation("LED_.*=180")
	for _, c := range []struct {
		rules []Rotation
		want  string
	}{
		{[]Rotation{specific, broad}, "D1,115.0000mm,-102.5400mm,Top,90\n"},
		{[]Rotation{broad, specific}, "D1,115.0000mm,-102.5400mm,Top,180\n"},
	} {
		got, err := CPL(jlc, []byte(posIn), c.rules)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), c.want) {
			t.Errorf("want %q in:\n%s", c.want, got)
		}
	}
}

func TestNormalize(t *testing.T) {
	for _, c := range [][2]float64{
		{0, 0}, {math.Copysign(0, -1), 0}, {360, 0}, {-360, 0}, {720, 0},
		{-90, 270}, {359.9999999, 0}, {-1e-16, 0}, {450, 90}, {12.5, 12.5},
	} {
		in, want := c[0], c[1]
		got := normalize(in)
		if got != want || math.Signbit(got) {
			t.Errorf("normalize(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestRotationMatchesWholeName(t *testing.T) {
	r, _ := ParseRotation("R_0603=90")
	got, err := CPL(jlc, []byte(posIn), []Rotation{r})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "R1,115.0000mm,-100.0000mm,Bottom,90\n") {
		t.Errorf("a pattern matched part of a name:\n%s", got)
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct {
		name string
		err  func() error
		want string
	}{
		{"bom column", func() error { _, err := BOM(jlc, []byte("Refs,Value\nR1,1k\n")); return err }, "bom: no column Footprint"},
		{"pos column", func() error { _, err := CPL(jlc, []byte("Ref,Val\nR1,1k\n"), nil); return err }, "pos: no column Package, PosX"},
		{"pos Val", func() error {
			_, err := CPL(pcbway, []byte("Ref,Package,PosX,PosY,Rot,Side\nR1,R,1,2,0,top\n"), nil)
			return err
		}, "pos: no column Val"},
		{"no refs", func() error { _, err := BOM(pcbway, []byte("Refs,Value,Footprint\n\" , \",1k,R\n")); return err }, "bom: a row with no Refs"},
		{"side", func() error {
			_, err := CPL(jlc, []byte("Ref,Val,Package,PosX,PosY,Rot,Side\nR1,1k,R,1,2,0,inner\n"), nil)
			return err
		}, `side "inner"`},
		{"rotation", func() error { _, err := ParseRotation("R_.*"); return err }, "want FOOTPRINT_REGEXP=DEGREES"},
		{"degrees", func() error { _, err := ParseRotation("R_.*=right"); return err }, "is not a number of degrees"},
		{"regexp", func() error { _, err := ParseRotation("R_(=90"); return err }, "missing closing )"},
	} {
		err := c.err()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}

func zipOf(t *testing.T, fab []File) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := Zip(&b, jlc, "main", fab, []byte("bom\n"), []byte("cpl\n")); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestZip(t *testing.T) {
	fab := []File{
		{"gerbers/main-F_Cu.gtl", []byte("top")},
		{"gerbers/main-job.gbrjob", []byte("job")},
		{"drill/main-drl.rpt", []byte("report")},
		{"drill/main-drl_map.PDF", []byte("map")},
		{"drill/main-drl_map.svg", []byte("map")},
		{"drill/main.drl", []byte("drill")},
		{"gerbers/main-Edge_Cuts.gm1", []byte("edge")},
	}
	b := zipOf(t, fab)
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range z.File {
		names = append(names, f.Name)
		if !f.Modified.Equal(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("%s: dated %v", f.Name, f.Modified)
		}
		if f.Mode().Perm() != 0o644 {
			t.Errorf("%s: mode %v", f.Name, f.Mode())
		}
	}
	if got, want := strings.Join(names, " "), "main-Edge_Cuts.gm1 main-F_Cu.gtl main.drl main_bom.csv main_cpl.csv"; got != want {
		t.Errorf("entries %q, want %q", got, want)
	}
	r, _ := z.File[2].Open()
	data, _ := io.ReadAll(r)
	if string(data) != "drill" {
		t.Errorf("main.drl holds %q", data)
	}
	// The same files in another order give the same bytes.
	fab[0], fab[3] = fab[3], fab[0]
	if !bytes.Equal(b, zipOf(t, fab)) {
		t.Error("the ZIP depends on the order of its inputs")
	}
}

func TestZipErrors(t *testing.T) {
	gtl := []File{{"g/main-F_Cu.gtl", []byte("x")}}
	for _, c := range []struct {
		board string
		fab   []File
		want  string
	}{
		{"main", nil, "no Gerber or drill file"},
		{"main", []File{{"g/main-job.gbrjob", nil}}, "no Gerber or drill file"},
		{"prod/main", gtl, "want a plain file name"},
		{"", gtl, "want a plain file name"},
	} {
		var b bytes.Buffer
		err := Zip(&b, jlc, c.board, c.fab, nil, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Zip(%q) = %v, want %q", c.board, err, c.want)
		}
	}
}

func TestZipDuplicateName(t *testing.T) {
	var b bytes.Buffer
	err := Zip(&b, jlc, "main", []File{{"a/x.gtl", nil}, {"b/x.gtl", nil}}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "two files named x.gtl") {
		t.Errorf("Zip = %v", err)
	}
}

var pcbway = Layouts["pcbway"]

func TestPCBWayBOM(t *testing.T) {
	in := `"Refs","Value","Footprint","Qty","DNP","MPN","Manufacturer"
"D1","LED","LED_SMD:LED_0603_1608Metric","1","","LTST-C191KRKT","Lite-On"
"R1,R2","1k","Resistor_SMD:R_0603_1608Metric","2","","",""
"R3","10k","Resistor_SMD:R_0603_1608Metric","1","DNP","",""
`
	got, err := BOM(pcbway, []byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "Item #,Designator,Qty,Manufacturer,Mfg Part #,Description / Value,Package/Footprint,Type,Your Instructions / Notes\n" +
		"1,D1,1,Lite-On,LTST-C191KRKT,LED,LED_0603_1608Metric,,\n" +
		"2,\"R1,R2\",2,,,1k,R_0603_1608Metric,,\n"
	if string(got) != want {
		t.Errorf("BOM:\n%s\nwant:\n%s", got, want)
	}
}

func TestQtyFromRefs(t *testing.T) {
	got, err := BOM(pcbway, []byte("Refs,Value,Footprint\n\"C1,C2,C3\",100n,C_0402\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "1,\"C1,C2,C3\",3,") {
		t.Errorf("no Qty 3 in:\n%s", got)
	}
}

func TestPCBWayCPL(t *testing.T) {
	got, err := CPL(pcbway, []byte(posIn), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Designator,Footprint,Mid X,Mid Y,TB,Rotation,Comment\n" +
		"D1,LED_0603_1608Metric,115.0000mm,-102.5400mm,T,0,LED\n" +
		"J1,PinHeader_1x02,105.0000mm,-100.0000mm,T,270,Power\n" +
		"R1,R_0603_1608Metric,115.0000mm,-100.0000mm,B,90,1k\n"
	if string(got) != want {
		t.Errorf("CPL:\n%s\nwant:\n%s", got, want)
	}
}

func TestCPLFootprintWithoutLibrary(t *testing.T) {
	got, err := CPL(pcbway, []byte("Ref,Val,Package,PosX,PosY,Rot,Side\nR1,1k,Resistor_SMD:R_0603,1,2,0,top\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "R1,R_0603,") {
		t.Errorf("library kept:\n%s", got)
	}
}

func TestPCBWayNames(t *testing.T) {
	if got := pcbway.BOMName("main") + " " + pcbway.CPLName("main"); got != "main_bom.csv main_centroid.csv" {
		t.Errorf("names %q", got)
	}
}

func TestZipBare(t *testing.T) {
	fab := []File{{"g/main-F_Cu.gtl", []byte("top")}, {"d/main.drl", []byte("drill")}}
	var b bytes.Buffer
	if err := Zip(&b, jlc, "main", fab, nil, nil); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range z.File {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, " "); got != "main-F_Cu.gtl main.drl" {
		t.Errorf("a bare ZIP holds %q", got)
	}
	// Empty lists are no lists: still bare.
	if err := Zip(&b, jlc, "main", fab, []byte{}, []byte{}); err != nil {
		t.Errorf("empty lists: %v", err)
	}
	for _, c := range []struct{ bom, cpl []byte }{{[]byte("b"), nil}, {nil, []byte("c")}, {[]byte("b"), []byte{}}} {
		if err := Zip(&b, jlc, "main", fab, c.bom, c.cpl); err == nil || !strings.Contains(err.Error(), "give both, or neither") {
			t.Errorf("one list only: %v", err)
		}
	}
}
