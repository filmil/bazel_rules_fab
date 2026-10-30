// SPDX-License-Identifier: Apache-2.0

// Package packager writes the ZIP a fab house takes for a board: its
// Gerber and drill files, a bill of materials and a component placement
// list, laid out and named as the vendor wants them (issue #9).
//
// The inputs are rules_kicad's outputs: the Gerber and drill files of
// kicad_gerbers and kicad_drill, the CSV of kicad_bom (columns Refs, Value,
// Footprint, and DNP and LCSC when present) and the CSV of kicad_pos
// (Ref, Val, Package, PosX, PosY, Rot, Side).
//
// The ZIP's entries are sorted by name, dated 1980-01-01 00:00:00 UTC
// (the earliest a ZIP can hold), with mode 0644, so the same inputs give
// the same bytes.
package packager

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Part is one row of kicad_bom's CSV: a group of parts of one value and
// footprint.
type Part struct {
	Refs, Value string
	// Footprint is without its library: "R_0603_1608Metric".
	Footprint string
	// Qty is the number of parts: the Qty column, or the count of Refs.
	Qty string
	// Fields are all columns of the row, such as LCSC, MPN, Manufacturer.
	Fields map[string]string
}

// Place is one row of kicad_pos's CSV, converted: X and Y read like
// "115.0000mm", Rotation is in [0, 360).
type Place struct {
	Ref, Value, Footprint string
	X, Y                  string
	Bottom                bool
	Rotation              string
}

// Layout is what one vendor wants: the columns of its BOM and placement
// list, and how it names the files.
type Layout struct {
	Name string
	// BOMHeader and BOMRow make the BOM: its header, and a row per part
	// group; i counts from 1.
	BOMHeader []string
	BOMRow    func(i int, p Part) []string
	// CPLHeader and CPLRow make the placement list.
	CPLHeader []string
	CPLRow    func(p Place) []string
	// BOMName and CPLName name the two CSV files from the board's name.
	BOMName, CPLName func(board string) string
}

func side(p Place, top, bottom string) string {
	if p.Bottom {
		return bottom
	}
	return top
}

// Layouts are the vendors the packager knows, by name.
var Layouts = map[string]Layout{
	// JLCPCB's assembly upload: https://jlcpcb.com/help/article/81-How-to-generate-the-BOM-and-Centroid-file-from-KiCAD
	"jlcpcb": {
		Name:      "JLCPCB",
		BOMHeader: []string{"Comment", "Designator", "Footprint", "LCSC Part #"},
		BOMRow: func(_ int, p Part) []string {
			return []string{p.Value, p.Refs, p.Footprint, p.Fields["LCSC"]}
		},
		CPLHeader: []string{"Designator", "Mid X", "Mid Y", "Layer", "Rotation"},
		CPLRow: func(p Place) []string {
			return []string{p.Ref, p.X, p.Y, side(p, "Top", "Bottom"), p.Rotation}
		},
		BOMName: func(b string) string { return b + "_bom.csv" },
		CPLName: func(b string) string { return b + "_cpl.csv" },
	},
	// PCBWay's sample BOM (Sample_BOM_PCBWay.xlsx) and centroid file
	// (a-sample-of-PCBWay-Centroid-File.txt), from
	// https://www.pcbway.com/helpcenter/pcb_assembly_ordering/What_files_are_requested_for_assembly_production_.html
	// The centroid columns Ref X/Y and Pad X/Y are left out: kicad_pos
	// does not give them, and PCBWay asks for X, Y, angle and designator.
	"pcbway": {
		Name: "PCBWay",
		BOMHeader: []string{"Item #", "Designator", "Qty", "Manufacturer", "Mfg Part #",
			"Description / Value", "Package/Footprint", "Type", "Your Instructions / Notes"},
		BOMRow: func(i int, p Part) []string {
			return []string{strconv.Itoa(i), p.Refs, p.Qty, p.Fields["Manufacturer"], p.Fields["MPN"],
				p.Value, p.Footprint, "", ""}
		},
		CPLHeader: []string{"Designator", "Footprint", "Mid X", "Mid Y", "TB", "Rotation", "Comment"},
		CPLRow: func(p Place) []string {
			return []string{p.Ref, p.Footprint, p.X, p.Y, side(p, "T", "B"), p.Rotation, p.Value}
		},
		BOMName: func(b string) string { return b + "_bom.csv" },
		CPLName: func(b string) string { return b + "_centroid.csv" },
	},
}

// Rotation corrects the rotation of the parts whose footprint matches
// Pattern by Degrees: a vendor's zero is its own model's orientation,
// which is not always KiCad's.
type Rotation struct {
	Pattern *regexp.Regexp
	Degrees float64
}

// ParseRotation reads "REGEXP=DEGREES".
func ParseRotation(s string) (Rotation, error) {
	i := strings.LastIndex(s, "=")
	if i <= 0 {
		return Rotation{}, fmt.Errorf("rotation %q: want FOOTPRINT_REGEXP=DEGREES", s)
	}
	re, err := regexp.Compile("^(?:" + s[:i] + ")$")
	if err != nil {
		return Rotation{}, fmt.Errorf("rotation %q: %v", s, err)
	}
	d, err := strconv.ParseFloat(s[i+1:], 64)
	if err != nil || math.IsNaN(d) || math.IsInf(d, 0) {
		return Rotation{}, fmt.Errorf("rotation %q: %q is not a number of degrees", s, s[i+1:])
	}
	return Rotation{re, d}, nil
}

// File is one input file: its name in the ZIP, and its bytes.
type File struct {
	Name string
	Data []byte
}

// readCSV reads a CSV with a header row; it returns the rows as maps from
// column to value.
func readCSV(what string, data []byte) ([]map[string]string, []string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %v", what, err)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("%s: no header row", what)
	}
	header := rows[0]
	var out []map[string]string
	for _, row := range rows[1:] {
		m := map[string]string{}
		for i, h := range header {
			if i < len(row) {
				m[h] = row[i]
			}
		}
		out = append(out, m)
	}
	return out, header, nil
}

func need(what string, header []string, cols ...string) error {
	have := map[string]bool{}
	for _, h := range header {
		have[h] = true
	}
	var missing []string
	for _, c := range cols {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: no column %s (it has %s)", what, strings.Join(missing, ", "), strings.Join(header, ", "))
	}
	return nil
}

func writeCSV(header []string, rows [][]string) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	if err := w.WriteAll(rows); err != nil { // flushes
		return nil, err
	}
	return b.Bytes(), nil
}

// BOM converts kicad_bom's CSV to the vendor's: one row per group, parts
// marked DNP left out, the footprint without its library.
func BOM(l Layout, data []byte) ([]byte, error) {
	rows, header, err := readCSV("bom", data)
	if err != nil {
		return nil, err
	}
	if err := need("bom", header, "Refs", "Value", "Footprint"); err != nil {
		return nil, err
	}
	var out [][]string
	for _, r := range rows {
		if strings.TrimSpace(r["DNP"]) != "" {
			continue
		}
		n := 0
		for _, ref := range strings.Split(r["Refs"], ",") {
			if strings.TrimSpace(ref) != "" {
				n++
			}
		}
		if n == 0 {
			return nil, fmt.Errorf("bom: a row with no Refs (value %q)", r["Value"])
		}
		qty := r["Qty"]
		if qty == "" {
			qty = strconv.Itoa(n)
		}
		out = append(out, l.BOMRow(len(out)+1, Part{
			Refs: r["Refs"], Value: r["Value"], Footprint: noLibrary(r["Footprint"]), Qty: qty, Fields: r,
		}))
	}
	return writeCSV(l.BOMHeader, out)
}

// CPL converts kicad_pos's CSV to the vendor's placement list, in mm, with
// each footprint's rotation corrected by the first rule that matches it.
func CPL(l Layout, data []byte, rotations []Rotation) ([]byte, error) {
	rows, header, err := readCSV("pos", data)
	if err != nil {
		return nil, err
	}
	if err := need("pos", header, "Ref", "Val", "Package", "PosX", "PosY", "Rot", "Side"); err != nil {
		return nil, err
	}
	var out [][]string
	for _, r := range rows {
		var bottom bool
		switch r["Side"] {
		case "top":
		case "bottom":
			bottom = true
		default:
			return nil, fmt.Errorf("pos: %s: side %q is neither top nor bottom", r["Ref"], r["Side"])
		}
		rot, err := strconv.ParseFloat(r["Rot"], 64)
		if err != nil {
			return nil, fmt.Errorf("pos: %s: rotation %q: %v", r["Ref"], r["Rot"], err)
		}
		for _, c := range rotations {
			if c.Pattern.MatchString(r["Package"]) {
				rot += c.Degrees
				break
			}
		}
		rot = normalize(rot)
		x, err := mm(r["PosX"])
		if err != nil {
			return nil, fmt.Errorf("pos: %s: x: %v", r["Ref"], err)
		}
		y, err := mm(r["PosY"])
		if err != nil {
			return nil, fmt.Errorf("pos: %s: y: %v", r["Ref"], err)
		}
		out = append(out, l.CPLRow(Place{
			Ref: r["Ref"], Value: r["Val"], Footprint: noLibrary(r["Package"]), X: x, Y: y,
			Bottom: bottom, Rotation: strconv.FormatFloat(rot, 'f', -1, 64),
		}))
	}
	return writeCSV(l.CPLHeader, out)
}

// noLibrary drops a footprint's library: "Resistor_SMD:R_0603" is
// "R_0603".
func noLibrary(fp string) string {
	if i := strings.Index(fp, ":"); i >= 0 {
		return fp[i+1:]
	}
	return fp
}

// normalize brings an angle into [0, 360), rounded to 1e-6 degrees, so
// -0, -360 and 359.9999999 all read 0.
func normalize(rot float64) float64 {
	rot = math.Round(math.Mod(rot, 360)*1e6) / 1e6
	if rot < 0 {
		rot += 360
	}
	if rot >= 360 || rot == 0 {
		rot = 0 // also turns -0 into 0
	}
	return rot
}

func mm(v string) (string, error) {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(f, 'f', 4, 64) + "mm", nil
}

// fabFile tells whether a file of kicad_gerbers or kicad_drill goes to the
// fab: Gerber layers and drill files, not KiCad's job file (.gbrjob),
// reports (.rpt) or drill maps (.pdf, .svg).
func fabFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".gbrjob", ".rpt", ".pdf", ".svg":
		return false
	}
	return true
}

// Zip writes the vendor's ZIP of the fab files and the two CSVs. File
// names use "/" as the separator.
func Zip(w io.Writer, l Layout, board string, fab []File, bom, cpl []byte) error {
	if board == "" || strings.ContainsAny(board, `/\`) {
		return fmt.Errorf("board name %q: want a plain file name", board)
	}
	files := []File{{l.BOMName(board), bom}, {l.CPLName(board), cpl}}
	n := 0
	for _, f := range fab {
		if fabFile(f.Name) {
			files = append(files, File{path.Base(f.Name), f.Data})
			n++
		}
	}
	if n == 0 {
		return fmt.Errorf("no Gerber or drill file to package")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for i := 1; i < len(files); i++ {
		if files[i].Name == files[i-1].Name {
			return fmt.Errorf("two files named %s", files[i].Name)
		}
	}
	z := zip.NewWriter(w)
	when := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: when}
		h.SetMode(0o644)
		fw, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := fw.Write(f.Data); err != nil {
			return err
		}
	}
	return z.Close()
}
