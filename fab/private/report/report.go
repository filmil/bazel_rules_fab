// SPDX-License-Identifier: Apache-2.0

// Package report makes fab_report's human-readable form of a fab
// package (issue #43): a PNG of each Gerber layer and drill file, the
// board's top and bottom as the fab sees them, and a Markdown report.
// Everything is drawn from the package's own files, so the pictures are
// of what is sent, not of the KiCad source.
package report

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/pkgcheck"
)

// Options says what to report.
type Options struct {
	Vendor  string
	Bare    bool
	PxPerMM int
	// Name is the package's file name, for the report's title.
	Name string
}

// What each Protel extension holds, in the order the report lists them.
var layerNames = []struct{ ext, what string }{
	{".gtl", "top copper"},
	{".gts", "top solder mask (openings)"},
	{".gto", "top silkscreen"},
	{".gtp", "top paste"},
	{".gbl", "bottom copper"},
	{".gbs", "bottom solder mask (openings)"},
	{".gbo", "bottom silkscreen"},
	{".gbp", "bottom paste"},
	{".gm1", "board outline"},
	{".drl", "drills"},
}

var innerExt = regexp.MustCompile(`^\.g([1-9][0-9]?)$`)

func what(ext string) string {
	for _, l := range layerNames {
		if l.ext == ext {
			return l.what
		}
	}
	if m := innerExt.FindStringSubmatch(ext); m != nil {
		return "inner copper " + m[1]
	}
	return ""
}

func order(ext string) int {
	for i, l := range layerNames {
		if l.ext == ext {
			return i
		}
	}
	if m := innerExt.FindStringSubmatch(ext); m != nil {
		n, _ := strconv.Atoi(m[1])
		return 100 + n
	}
	return 1000
}

// Colours.
var (
	white     = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	boardGrey = color.NRGBA{0xe6, 0xe6, 0xe6, 0xff}
	ink       = color.NRGBA{0x1a, 0x1a, 0x2e, 0xff}
	edge      = color.NRGBA{0xc0, 0x30, 0x30, 0xff}
	substrate = color.NRGBA{0xb8, 0xa0, 0x62, 0xff}
	maskBare  = color.NRGBA{0x1e, 0x5c, 0x2a, 0xff}
	maskCu    = color.NRGBA{0x3c, 0x8c, 0x46, 0xff}
	padCu     = color.NRGBA{0xc8, 0xa2, 0x3a, 0xff}
	silk      = color.NRGBA{0xf4, 0xf4, 0xf4, 0xff}
	hole      = color.NRGBA{0x20, 0x20, 0x20, 0xff}
)

// maxSamples bounds a mask (a byte a sample): 300 MB, a 250 x 250 mm board
// at the default 20 pixels per mm. A larger board takes a lower
// px_per_mm.
const maxSamples = 300_000_000

type file struct {
	name, ext string
	data      []byte
	layer     *Layer
	drill     *Drill
	mask      *Mask
}

// Make writes the report into dir: report.md, top.png, bottom.png and
// <file>.png for each Gerber and drill file of the ZIP.
func Make(zipData []byte, o Options, dir string) error {
	if o.PxPerMM < 1 || o.PxPerMM > 100 {
		return fmt.Errorf("pixels per mm %d: want 1 to 100", o.PxPerMM)
	}
	z, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("not a ZIP: %v", err)
	}
	var files []*file
	var others []*file
	for _, zf := range z.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		r, err := zf.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return fmt.Errorf("%s: %v", zf.Name, err)
		}
		f := &file{name: zf.Name, ext: strings.ToLower(path.Ext(zf.Name)), data: b}
		switch {
		case f.ext == ".drl":
			if f.drill, err = ParseExcellon(b); err != nil {
				return fmt.Errorf("%s: %v", f.name, err)
			}
			files = append(files, f)
		case what(f.ext) != "":
			if f.layer, err = ParseGerber(b); err != nil {
				return fmt.Errorf("%s: %v", f.name, err)
			}
			files = append(files, f)
		default:
			others = append(others, f)
		}
	}
	sort.SliceStable(files, func(i, j int) bool {
		if order(files[i].ext) != order(files[j].ext) {
			return order(files[i].ext) < order(files[j].ext)
		}
		return files[i].name < files[j].name
	})
	byExt := map[string]*file{}
	for _, f := range files {
		if _, ok := byExt[f.ext]; !ok {
			byExt[f.ext] = f
		}
	}

	// The board's shape, from the outline.
	var notes []string
	outline := byExt[".gm1"]
	var loops [][]Pt
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(p Pt) {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	if outline != nil && len(outline.layer.Path) == 0 {
		notes = append(notes, fmt.Sprintf("The outline, %s, has nothing on it: the board is drawn as the box of its layers.", outline.name))
	}
	if outline != nil {
		for _, s := range outline.layer.Path {
			grow(s[0])
			grow(s[1])
		}
		var err error
		loops, err = Loops(outline.layer.Path)
		if err != nil {
			notes = append(notes, fmt.Sprintf("The outline does not close (%v): the board is drawn as its bounding box.", err))
			loops = nil
		}
	} else {
		notes = append(notes, "The package has no board outline (*.gm1): the board is drawn as the box of its layers.")
	}
	if math.IsInf(minX, 1) {
		for _, f := range files {
			if f.layer != nil {
				for _, ob := range f.layer.Objects {
					a, b, c, d := bounds(ob.prims)
					grow(Pt{a, b})
					grow(Pt{c, d})
				}
			}
			if f.drill != nil {
				for _, t := range f.drill.Tools {
					r := t.Diameter / 2
					for _, h := range t.Holes {
						grow(Pt{h.X - r, h.Y - r})
						grow(Pt{h.X + r, h.Y + r})
					}
					for _, s := range t.Slots {
						for _, h := range s {
							grow(Pt{h.X - r, h.Y - r})
							grow(Pt{h.X + r, h.Y + r})
						}
					}
				}
			}
		}
	}
	outlined := loops != nil
	empty := math.IsInf(minX, 1)
	if empty {
		notes = append(notes, "Nothing to draw: the outline and every layer are empty.")
		minX, minY, maxX, maxY = 0, 0, 1, 1
	}
	frame := NewFrame(minX, minY, maxX, maxY, 1, o.PxPerMM)
	if n := int64(frame.W) * int64(frame.H) * SS * SS; n > maxSamples {
		return fmt.Errorf("the renders of a %s x %s mm board at %d pixels per mm need %d samples a layer, "+
			"more than %d: give a lower px_per_mm, or check the outline for a stray point",
			mmf(maxX-minX), mmf(maxY-minY), o.PxPerMM, n, int64(maxSamples))
	}
	board := NewMask(frame)
	if loops != nil {
		board.FillLoops(loops)
	} else {
		board.FillLoops([][]Pt{{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}}})
	}

	drills := NewMask(frame)
	for _, f := range files {
		if f.drill != nil {
			drills.DrawDrill(f.drill)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	save := func(name string, img image.Image) error {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644)
	}
	// One layer's mask at a time; only the composites' six stay.
	for _, f := range files {
		m := NewMask(frame)
		if f.drill != nil {
			m.DrawDrill(f.drill)
		} else {
			m.Draw(f.layer)
		}
		isEdge := f.ext == ".gm1"
		img := Paint(frame, false, func(at func(*Mask) bool) color.NRGBA {
			switch {
			case at(m) && isEdge:
				return edge
			case at(m):
				return ink
			case at(board):
				return boardGrey
			}
			return white
		})
		if err := save(pngName(f.name), img); err != nil {
			return err
		}
		switch f.ext {
		case ".gtl", ".gts", ".gto", ".gbl", ".gbs", ".gbo":
			if byExt[f.ext] == f {
				f.mask = m
			}
		}
	}
	for _, side := range []struct {
		name       string
		cu, sm, ss string
		mirror     bool
	}{
		{"top", ".gtl", ".gts", ".gto", false},
		{"bottom", ".gbl", ".gbs", ".gbo", true},
	} {
		var cu, sm, ss *Mask
		if f := byExt[side.cu]; f != nil {
			cu = f.mask
		}
		if f := byExt[side.sm]; f != nil {
			sm = f.mask
		}
		if f := byExt[side.ss]; f != nil {
			ss = f.mask
		}
		img := Paint(frame, side.mirror, func(at func(*Mask) bool) color.NRGBA {
			switch {
			case !at(board) || at(drills):
				if at(drills) && at(board) {
					return hole
				}
				return white
			case at(sm): // a mask opening: no mask, no silkscreen
				if at(cu) {
					return padCu
				}
				return substrate
			case at(ss):
				return silk
			case at(cu):
				return maskCu
			}
			return maskBare
		})
		if err := save(side.name+".png", img); err != nil {
			return err
		}
	}

	if empty {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}
	md, err := markdown(o, files, others, zipData, minX, minY, maxX, maxY, outlined, notes)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(md), 0o644)
}

func mmf(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}

func markdown(o Options, files, others []*file, zipData []byte, minX, minY, maxX, maxY float64, outlined bool,
	notes []string) (string, error) {
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	p("# %s\n\n", o.Name)
	p("The fab package for %s, drawn from its own files: what the fab gets.\n\n", o.Vendor)

	copper := 0
	for _, f := range files {
		if strings.HasPrefix(what(f.ext), "top copper") || strings.HasPrefix(what(f.ext), "bottom copper") ||
			strings.HasPrefix(what(f.ext), "inner copper") {
			copper++
		}
	}
	p("## Board\n\n")
	if maxX > minX || maxY > minY {
		box := "the outline's box"
		if !outlined {
			box = "the box of what is drawn"
		}
		p("* Size: %s x %s mm (%s).\n", mmf(maxX-minX), mmf(maxY-minY), box)
	}
	p("* Copper layers: %d.\n", copper)
	for _, n := range notes {
		p("* %s\n", n)
	}
	p("\n")

	p("## Renders\n\n")
	p("The top, and the bottom as seen from below (mirrored). Gold is copper in a\n")
	p("solder mask opening, light green copper under the mask, white silkscreen\n")
	p("(left out of mask openings, as fabs print it), dark grey a hole.\n\n")
	p("| Top | Bottom |\n|---|---|\n| ![top](top.png) | ![bottom](bottom.png) |\n\n")
	p("Each file, seen from the top, as drawn:\n\n")
	p("| File | Holds | Bytes | Render |\n|---|---|---:|---|\n")
	for _, f := range files {
		p("| `%s` | %s | %d | [%s](%s) |\n", cell(f.name), what(f.ext), len(f.data), cell(pngName(f.name)),
			url.PathEscape(pngName(f.name)))
	}
	for _, f := range others {
		p("| `%s` | %s | %d | |\n", cell(f.name), "table", len(f.data))
	}
	p("\n")

	p("## Drills\n\n")
	p("| File | Tool | Diameter (mm) | Plating | Holes | Slots |\n|---|---|---:|---|---:|---:|\n")
	holes, slots := 0, 0
	smallest := math.Inf(1)
	for _, f := range files {
		if f.drill == nil {
			continue
		}
		for _, t := range f.drill.Tools {
			pl := t.Plated
			if pl == "" {
				pl = "not given"
			}
			p("| `%s` | T%d | %s | %s | %d | %d |\n", cell(f.name), t.Code, strconv.FormatFloat(t.Diameter, 'f', 3, 64),
				pl, len(t.Holes), len(t.Slots))
			holes += len(t.Holes)
			slots += len(t.Slots)
			if len(t.Holes)+len(t.Slots) > 0 {
				smallest = math.Min(smallest, t.Diameter)
			}
		}
	}
	p("\n%s and %s", count(holes, "hole"), count(slots, "slot"))
	if !math.IsInf(smallest, 1) {
		p("; the smallest drill is %s mm", strconv.FormatFloat(smallest, 'f', 3, 64))
	}
	p(".\n\n")

	spec, ok := pkgcheck.Specs[o.Vendor]
	if !ok {
		return "", fmt.Errorf("vendor %q: not known", o.Vendor)
	}
	if o.Bare {
		p("## Parts\n\nA bare board: no BOM and no placement list.\n\n")
	} else {
		bom, cpl := find(others, spec.BOMSuffix), find(others, spec.CPLSuffix)
		bomRefs := map[string]bool{}
		p("## BOM\n\n")
		if bom == nil {
			p("No BOM (*%s).\n\n", spec.BOMSuffix)
		} else {
			rows, err := table(bom.data)
			if err != nil {
				return "", fmt.Errorf("%s: %v", bom.name, err)
			}
			parts := 0
			col := index(rows, spec.BOMRefs)
			for i, r := range rows {
				p("|")
				for _, c := range r {
					p(" %s |", cell(c))
				}
				p("\n")
				if i == 0 {
					p("|%s\n", strings.Repeat("---|", len(r)))
					continue
				}
				if col >= 0 && col < len(r) {
					for _, ref := range refs(r[col]) {
						bomRefs[ref] = true
						parts++
					}
				}
			}
			p("\n`%s`: %d lines, %d parts. Parts marked DNP are not in it.\n\n", bom.name, max(len(rows)-1, 0), parts)
		}
		p("## Placement\n\n")
		if cpl == nil {
			p("No placement list (*%s).\n\n", spec.CPLSuffix)
		} else {
			rows, err := table(cpl.data)
			if err != nil {
				return "", fmt.Errorf("%s: %v", cpl.name, err)
			}
			layer, ref := index(rows, spec.Layer), index(rows, spec.CPLRef)
			count := map[string]int{}
			var notInBOM []string
			placed := map[string]bool{}
			for _, r := range rows[min(1, len(rows)):] {
				if layer >= 0 && layer < len(r) {
					count[r[layer]]++
				}
				if ref >= 0 && ref < len(r) {
					placed[r[ref]] = true
					if bom != nil && !bomRefs[r[ref]] {
						notInBOM = append(notInBOM, r[ref])
					}
				}
			}
			p("| Side (`%s`) | Parts |\n|---|---:|\n", spec.Layer)
			for _, l := range spec.Layers {
				p("| %s | %d |\n", l, count[l])
			}
			p("\n")
			var unplaced []string
			for r := range bomRefs {
				if !placed[r] {
					unplaced = append(unplaced, r)
				}
			}
			sort.Strings(unplaced)
			sort.Strings(notInBOM)
			if len(notInBOM) > 0 {
				p("Placed but not in the BOM: %s.\n\n", strings.Join(notInBOM, ", "))
			}
			if len(unplaced) > 0 {
				p("In the BOM but not placed: %s.\n\n", strings.Join(unplaced, ", "))
			}
		}
	}

	p("## Checks\n\n")
	p("fab_package_test's checker on this ZIP, for %s: the files are at the ZIP's\n", o.Vendor)
	p("root; every layer the vendor needs is there once (copper, mask and\n")
	p("silkscreen of both sides, the outline, the drills); no file is empty or\n")
	p("one the vendor does not take; ")
	if o.Bare {
		p("there is no BOM or placement list, as for a bare board.\n\n")
	} else {
		p("there is one BOM and one placement list, of the same\n")
		p("board, with the vendor's columns; every row has a designator, positions\n")
		p("are in mm, rotations at least 0 and below 360, and sides one of %s.\n\n",
			strings.Join(spec.Layers, ", "))
	}
	problems, err := pkgcheck.Check(o.Vendor, zipData, o.Bare)
	if err != nil {
		return "", err
	}
	if len(problems) == 0 {
		p("Result: no problems.\n")
	} else {
		p("Result: %d problems:\n\n", len(problems))
		for _, pr := range problems {
			p("* %s\n", pr)
		}
	}
	return b.String(), nil
}

// pngName is a file's render's name: one plain file name in the output
// directory, whatever path the ZIP gives the file.
func pngName(name string) string {
	n := strings.NewReplacer("/", "_", "\\", "_").Replace(name)
	if strings.HasPrefix(n, ".") {
		n = "_" + n
	}
	return n + ".png"
}

func count(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

func find(fs []*file, suffix string) *file {
	for _, f := range fs {
		if strings.HasSuffix(f.name, suffix) {
			return f
		}
	}
	return nil
}

func table(data []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

func index(rows [][]string, col string) int {
	if len(rows) == 0 {
		return -1
	}
	for i, c := range rows[0] {
		if c == col {
			return i
		}
	}
	return -1
}

func refs(s string) []string {
	var out []string
	for _, r := range strings.FieldsFunc(s, func(c rune) bool { return c == ',' || c == ' ' }) {
		out = append(out, r)
	}
	return out
}
