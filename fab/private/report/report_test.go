// SPDX-License-Identifier: Apache-2.0

package report

import (
	"archive/zip"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const header = "%FSLAX46Y46*%\n%MOMM*%\n%LPD*%\n"

// KiCad 9's macro, as it writes it.
const roundRect = `%AMRoundRect*
0 Rectangle with rounded corners*
0 $1 Rounding radius*
0 $2 $3 $4 $5 $6 $7 $8 $9 X,Y pos of 4 corners*
0 Add a 4 corners polygon primitive as box body*
4,1,4,$2,$3,$4,$5,$6,$7,$8,$9,$2,$3,0*
0 Add four circle primitives for the rounded corners*
1,1,$1+$1,$2,$3*
1,1,$1+$1,$4,$5*
1,1,$1+$1,$6,$7*
1,1,$1+$1,$8,$9*
0 Add four rect primitives between the rounded corners*
20,1,$1+$1,$2,$3,$4,$5,0*
20,1,$1+$1,$4,$5,$6,$7,0*
20,1,$1+$1,$6,$7,$8,$9,0*
20,1,$1+$1,$8,$9,$2,$3,0*%
`

// area draws a Gerber on a frame of the box and returns its dark area in
// mm².
func area(t *testing.T, gerber string, minX, minY, maxX, maxY float64) float64 {
	t.Helper()
	l, err := ParseGerber([]byte(gerber))
	if err != nil {
		t.Fatalf("ParseGerber: %v", err)
	}
	m := NewMask(NewFrame(minX, minY, maxX, maxY, 0.5, 50))
	m.Draw(l)
	return dark(m)
}

func dark(m *Mask) float64 {
	n := 0
	for _, b := range m.bits {
		if b {
			n++
		}
	}
	s := m.scale()
	return float64(n) / (s * s)
}

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol*want {
		t.Errorf("%s: area %.4f mm², want %.4f within %.0f%%", what, got, want, tol*100)
	}
}

func TestExpr(t *testing.T) {
	vars := map[int]float64{1: 0.2, 2: 3}
	for in, want := range map[string]float64{
		"$1+$1":      0.4,
		"-$2x2":      -6,
		"(1+2)x$2/3": 3,
		"2-1-1":      0,
		"$7":         0,
		"1.5":        1.5,
	} {
		got, err := evalExpr(in, vars)
		if err != nil || math.Abs(got-want) > 1e-12 {
			t.Errorf("evalExpr(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := evalExpr("1+", vars); err == nil {
		t.Error("evalExpr(1+): no error")
	}
}

func TestApertures(t *testing.T) {
	near(t, "C flash", area(t, header+"%ADD10C,1.000000*%\nD10*\nX0Y0D03*\nM02*\n", -1, -1, 1, 1),
		math.Pi/4, 0.02)
	near(t, "R flash", area(t, header+"%ADD10R,2.000000X1.000000*%\nD10*\nX0Y0D03*\nM02*\n", -1, -1, 1, 1),
		2, 0.02)
	near(t, "O flash", area(t, header+"%ADD10O,2.000000X1.000000*%\nD10*\nX0Y0D03*\nM02*\n", -1, -1, 1, 1),
		1+math.Pi/4, 0.02)
	near(t, "C flash with a hole", area(t, header+"%ADD10C,2.000000X1.000000*%\nD10*\nX0Y0D03*\nM02*\n", -1, -1, 1, 1),
		math.Pi*(1-0.25), 0.02)
	// KiCad's pad: corners at (±0.2, ±0.275), radius 0.2.
	want := 0.8*0.95 - (4-math.Pi)*0.04
	near(t, "RoundRect", area(t, header+roundRect+
		"%ADD10RoundRect,0.200000X-0.200000X-0.275000X0.200000X-0.275000X0.200000X0.275000X-0.200000X0.275000X0*%\n"+
		"D10*\nX1000000Y0D03*\nM02*\n", 0, -1, 2, 1), want, 0.02)
}

func TestStrokes(t *testing.T) {
	// A 0.2 mm line 10 mm long: a 10 x 0.2 band and two half circles.
	near(t, "line", area(t, header+"%ADD10C,0.200000*%\nD10*\nX0Y0D02*\nX10000000Y0D01*\nM02*\n", 0, -1, 10, 1),
		2+math.Pi*0.01, 0.03)
	// A full circle of radius 2, 0.2 wide.
	l, err := ParseGerber([]byte(header + "%ADD10C,0.200000*%\nD10*\nG75*\nX2000000Y0D02*\nG03*\nX2000000Y0I-2000000J0D01*\nM02*\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewMask(NewFrame(-3, -3, 3, 3, 0, 50))
	m.Draw(l)
	near(t, "full circle", dark(m), 2*math.Pi*2*0.2, 0.03)
	loops, err := Loops(l.Path)
	if err != nil || len(loops) != 1 {
		t.Errorf("Loops of a circle: %d loops, %v", len(loops), err)
	}
	// A rectangular aperture's stroke: the hull of both ends.
	near(t, "R stroke", area(t, header+"%ADD10R,1.000000X1.000000*%\nD10*\nX0Y0D02*\nX4000000Y0D01*\nM02*\n", -1, -1, 5, 1),
		5, 0.02)
}

func TestRegionAndClear(t *testing.T) {
	sq := "G36*\nX0Y0D02*\nG01*\nX4000000Y0D01*\nX4000000Y4000000D01*\nX0Y4000000D01*\nX0Y0D01*\nG37*\n"
	near(t, "region", area(t, header+sq+"M02*\n", 0, 0, 4, 4), 16, 0.01)
	clear := header + sq + "%LPC*%\n%ADD10C,2.000000*%\nD10*\nX2000000Y2000000D03*\nM02*\n"
	near(t, "region with a clear flash", area(t, clear, 0, 0, 4, 4), 16-math.Pi, 0.01)
}

func TestRefuses(t *testing.T) {
	for what, g := range map[string]string{
		"G74":          header + "G74*\nM02*\n",
		"G91":          header + "G91*\nM02*\n",
		"step":         header + "%SRX2Y2I1J1*%\nM02*\n",
		"mirror":       header + "%LMX*%\nM02*\n",
		"no aperture":  header + "X0Y0D03*\nM02*\n",
		"open region":  header + "G36*\nX0Y0D02*\n",
		"bad format":   "%FSTAX24Y24*%\n",
		"unknown prim": header + "%AMX*\n9,1*%\n%ADD10X*%\nM02*\n",
	} {
		if _, err := ParseGerber([]byte(g)); err == nil {
			t.Errorf("%s: no error", what)
		}
	}
}

const drill = `M48
; #@! TF.FileFunction,MixedPlating,1,2
FMAT,2
METRIC
; #@! TA.AperFunction,Plated,PTH,ViaDrill
T1C0.300
; #@! TA.AperFunction,NonPlated,NPTH,ComponentDrill
T2C2.700
; #@! TA.AperFunction,Plated,PTH,ComponentDrill
T3C0.600
%
G90
G05
T1
X1.0Y-1.0
X2.0Y-1.0
T2
X5.0Y-5.0
T3
X3.0Y-8.18G85X2.2Y-8.18
G05
M30
`

func TestExcellon(t *testing.T) {
	d, err := ParseExcellon([]byte(drill))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Tools) != 3 {
		t.Fatalf("%d tools, want 3", len(d.Tools))
	}
	for i, want := range []struct {
		dia          float64
		plated       string
		holes, slots int
	}{{0.3, "PTH", 2, 0}, {2.7, "NPTH", 1, 0}, {0.6, "PTH", 0, 1}} {
		got := d.Tools[i]
		if got.Diameter != want.dia || got.Plated != want.plated || len(got.Holes) != want.holes || len(got.Slots) != want.slots {
			t.Errorf("T%d: %v %s %d holes %d slots; want %+v", got.Code, got.Diameter, got.Plated,
				len(got.Holes), len(got.Slots), want)
		}
	}
	if s := d.Tools[2].Slots[0]; s[0] != (Pt{3, -8.18}) || s[1] != (Pt{2.2, -8.18}) {
		t.Errorf("slot %v", s)
	}
	if _, err := ParseExcellon([]byte("M48\nMETRIC\nT1C0.3\n%\nT1\nX100Y200\n")); err == nil {
		t.Error("coordinates with no decimal point: no error")
	}
}

func TestLoopsCutout(t *testing.T) {
	seg := func(ax, ay, bx, by float64) [2]Pt { return [2]Pt{{ax, ay}, {bx, by}} }
	// A 10 x 10 board with a 2 x 2 cut-out; the segments out of order and
	// some reversed, as a file may have them.
	segs := [][2]Pt{
		seg(0, 0, 10, 0), seg(4, 4, 6, 4), seg(10, 10, 10, 0), seg(6, 4, 6, 6),
		seg(0, 10, 10, 10), seg(4, 6, 6, 6), seg(0, 0, 0, 10), seg(4, 4, 4, 6),
	}
	loops, err := Loops(segs)
	if err != nil || len(loops) != 2 {
		t.Fatalf("Loops: %d loops, %v", len(loops), err)
	}
	m := NewMask(NewFrame(0, 0, 10, 10, 1, 20))
	m.FillLoops(loops)
	near(t, "board with a cut-out", dark(m), 96, 0.01)
	if _, err := Loops(segs[:3]); err == nil {
		t.Error("an open outline: no error")
	}
}

func gerberFile(function, body string) string {
	return "%TF.FileFunction," + function + "*%\n" + header + body + "M02*\n"
}

func testZip(t *testing.T, bare bool) []byte {
	t.Helper()
	outline := "%ADD10C,0.050000*%\nD10*\nX0Y0D02*\nX20000000Y0D01*\nX20000000Y10000000D01*\nX0Y10000000D01*\nX0Y0D01*\n"
	pad := "%ADD10R,1.000000X1.000000*%\nD10*\nX5000000Y5000000D03*\nX15000000Y5000000D03*\n"
	files := map[string]string{
		"b-F_Cu.gtl":         gerberFile("Copper,L1,Top", pad),
		"b-B_Cu.gbl":         gerberFile("Copper,L2,Bot", pad),
		"b-F_Mask.gts":       gerberFile("Soldermask,Top", pad),
		"b-B_Mask.gbs":       gerberFile("Soldermask,Bot", ""),
		"b-F_Silkscreen.gto": gerberFile("Legend,Top", "%ADD10C,0.150000*%\nD10*\nX2000000Y2000000D02*\nX8000000Y2000000D01*\n"),
		"b-B_Silkscreen.gbo": gerberFile("Legend,Bot", ""),
		"b-Edge_Cuts.gm1":    gerberFile("Profile,NP", outline),
		"b.drl":              drill,
	}
	if !bare {
		files["b_bom.csv"] = "Comment,Designator,Footprint,LCSC Part #\n1u,\"C1,C2\",C_0603,C15849\n"
		files["b_cpl.csv"] = "Designator,Mid X,Mid Y,Layer,Rotation\nC1,5.0000mm,5.0000mm,Top,0\nC2,15.0000mm,5.0000mm,Bottom,90\nR9,1.0000mm,1.0000mm,Top,0\n"
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range []string{"b-B_Cu.gbl", "b-B_Mask.gbs", "b-B_Silkscreen.gbo", "b-Edge_Cuts.gm1",
		"b-F_Cu.gtl", "b-F_Mask.gts", "b-F_Silkscreen.gto", "b.drl", "b_bom.csv", "b_cpl.csv"} {
		body, ok := files[name]
		if !ok {
			continue
		}
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMake(t *testing.T) {
	data := testZip(t, false)
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, d := range []string{a, b} {
		if err := Make(data, Options{Vendor: "jlcpcb", PxPerMM: 10, Name: "b.zip"}, d); err != nil {
			t.Fatalf("Make: %v", err)
		}
	}
	md, err := os.ReadFile(filepath.Join(a, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# b.zip",
		"Size: 20.00 x 10.00 mm",
		"Copper layers: 2.",
		"| `b.drl` | T2 | 2.700 | NPTH | 1 | 0 |",
		"3 holes and 1 slot; the smallest drill is 0.300 mm.",
		"| 1u | C1,C2 | C_0603 | C15849 |",
		"1 lines, 2 parts",
		"| Top | 2 |",
		"| Bottom | 1 |",
		"Placed but not in the BOM: R9.",
		"Result: no problems.",
		"[b-F_Cu.gtl.png](b-F_Cu.gtl.png)",
	} {
		if !strings.Contains(string(md), want) {
			t.Errorf("report.md has no %q:\n%s", want, md)
		}
	}
	for _, name := range []string{"top.png", "bottom.png", "b-F_Cu.gtl.png", "b.drl.png", "b-Edge_Cuts.gm1.png", "report.md"} {
		x, err := os.ReadFile(filepath.Join(a, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		y, _ := os.ReadFile(filepath.Join(b, name))
		if !bytes.Equal(x, y) {
			t.Errorf("%s: two runs differ", name)
		}
	}
}

func TestMakeBare(t *testing.T) {
	dir := t.TempDir()
	if err := Make(testZip(t, true), Options{Vendor: "jlcpcb", Bare: true, PxPerMM: 5, Name: "b.zip"}, dir); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(filepath.Join(dir, "report.md"))
	for _, want := range []string{"A bare board: no BOM and no placement list.", "Result: no problems."} {
		if !strings.Contains(string(md), want) {
			t.Errorf("report.md has no %q:\n%s", want, md)
		}
	}
	// The checker's problems are listed, not hidden: a bare package read
	// as one with parts lacks its BOM and placement list.
	if err := Make(testZip(t, true), Options{Vendor: "jlcpcb", PxPerMM: 5, Name: "b.zip"}, dir); err != nil {
		t.Fatal(err)
	}
	md, _ = os.ReadFile(filepath.Join(dir, "report.md"))
	if !strings.Contains(string(md), "Result: 2 problems:") {
		t.Errorf("report.md does not list the 2 problems:\n%s", md)
	}
}
