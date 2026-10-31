// SPDX-License-Identifier: Apache-2.0

// Package pkgcheck checks a fab house's ZIP, as fab_package writes it,
// against what the vendor takes (issue #11): the layer set, the file
// names, and the columns and values of the BOM and placement list.
//
// Its vendor table is its own, written from the vendors' documents, not
// the packager's: a packager bug then fails the check.
package pkgcheck

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
)

// Spec is what one vendor takes.
type Spec struct {
	// BOMSuffix and CPLSuffix end the names of the BOM and placement list:
	// "<board><suffix>".
	BOMSuffix, CPLSuffix string
	BOMHeader, CPLHeader []string
	// Designator columns, in the BOM and in the placement list.
	BOMRefs, CPLRef string
	// X, Y, Rotation and Layer are the placement list's columns; Layers
	// the values its Layer column takes.
	X, Y, Rotation, Layer string
	Layers                []string
}

// Specs are the vendors, by name.
var Specs = map[string]Spec{
	// https://jlcpcb.com/help/article/81-How-to-generate-the-BOM-and-Centroid-file-from-KiCAD
	"jlcpcb": {
		BOMSuffix: "_bom.csv",
		CPLSuffix: "_cpl.csv",
		BOMHeader: []string{"Comment", "Designator", "Footprint", "LCSC Part #"},
		CPLHeader: []string{"Designator", "Mid X", "Mid Y", "Layer", "Rotation"},
		BOMRefs:   "Designator",
		CPLRef:    "Designator",
		X:         "Mid X", Y: "Mid Y", Rotation: "Rotation", Layer: "Layer",
		Layers: []string{"Top", "Bottom"},
	},
	// PCBWay's sample BOM and centroid file, from
	// https://www.pcbway.com/helpcenter/pcb_assembly_ordering/What_files_are_requested_for_assembly_production_.html
	"pcbway": {
		BOMSuffix: "_bom.csv",
		CPLSuffix: "_centroid.csv",
		BOMHeader: []string{"Item #", "Designator", "Qty", "Manufacturer", "Mfg Part #",
			"Description / Value", "Package/Footprint", "Type", "Your Instructions / Notes"},
		CPLHeader: []string{"Designator", "Footprint", "Mid X", "Mid Y", "TB", "Rotation", "Comment"},
		BOMRefs:   "Designator",
		CPLRef:    "Designator",
		X:         "Mid X", Y: "Mid Y", Rotation: "Rotation", Layer: "TB",
		Layers: []string{"T", "B"},
	},
}

// The layers every board needs, by Protel extension, and the drill file.
var required = []struct{ ext, what string }{
	{".gtl", "top copper (F_Cu)"},
	{".gbl", "bottom copper (B_Cu)"},
	{".gts", "top solder mask (F_Mask)"},
	{".gbs", "bottom solder mask (B_Mask)"},
	{".gto", "top silkscreen (F_Silkscreen)"},
	{".gbo", "bottom silkscreen (B_Silkscreen)"},
	{".gm1", "board outline (Edge_Cuts)"},
	{".drl", "drill file"},
}

// optional are the other files a fab takes: paste layers and inner copper
// (.g1 to .g30).
var optional = regexp.MustCompile(`^\.(gtp|gbp|g([1-9]|[12][0-9]|30))$`)

var mmValue = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?mm$`)

// Check checks the ZIP's bytes against the vendor's spec. It returns every
// problem it finds, sorted; none when the ZIP is good. A bare board's ZIP
// (bare) must have no BOM and no placement list; any other must have one
// of each.
func Check(vendor string, data []byte, bare bool) ([]string, error) {
	s, ok := Specs[vendor]
	if !ok {
		var names []string
		for k := range Specs {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("vendor %q: want one of %s", vendor, strings.Join(names, ", "))
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a ZIP: %v", err)
	}
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	files := map[string][]byte{}
	have := map[string][]string{}
	var boms, cpls []string
	for _, f := range z.File {
		if strings.Contains(f.Name, "/") {
			add("%s: not at the ZIP's root", f.Name)
			continue
		}
		b, err := read(f)
		if err != nil {
			return nil, err
		}
		files[f.Name] = b
		ext := strings.ToLower(path.Ext(f.Name))
		switch {
		case strings.HasSuffix(f.Name, s.CPLSuffix):
			if f.Name == s.CPLSuffix {
				add("%s: no board name before %s", f.Name, s.CPLSuffix)
			}
			cpls = append(cpls, f.Name)
		case strings.HasSuffix(f.Name, s.BOMSuffix):
			if f.Name == s.BOMSuffix {
				add("%s: no board name before %s", f.Name, s.BOMSuffix)
			}
			boms = append(boms, f.Name)
		case optional.MatchString(ext):
		default:
			known := false
			for _, r := range required {
				if ext == r.ext {
					known = true
				}
			}
			if !known {
				add("%s: not a file %s takes", f.Name, vendor)
				continue
			}
		}
		if len(b) == 0 {
			add("%s: empty", f.Name)
		}
		have[ext] = append(have[ext], f.Name)
	}
	sort.Strings(boms)
	sort.Strings(cpls)
	for _, r := range required {
		switch n := have[r.ext]; {
		case len(n) == 0:
			add("no %s (*%s)", r.what, r.ext)
		case len(n) > 1 && r.ext != ".drl": // plated and unplated drill files may be apart
			add("two files of %s: %s", r.what, strings.Join(n, ", "))
		}
	}
	switch {
	case bare:
		for _, n := range append(append([]string{}, boms...), cpls...) {
			add("%s: a bare board's ZIP has no BOM or placement list", n)
		}
	case len(boms) == 0:
		add("no BOM (*%s)", s.BOMSuffix)
	case len(boms) > 1:
		add("two BOMs: %s", strings.Join(boms, ", "))
	}
	switch {
	case bare:
	case len(cpls) == 0:
		add("no placement list (*%s)", s.CPLSuffix)
	case len(cpls) > 1:
		add("two placement lists: %s", strings.Join(cpls, ", "))
	}
	if len(boms) == 1 && len(cpls) == 1 &&
		strings.TrimSuffix(boms[0], s.BOMSuffix) != strings.TrimSuffix(cpls[0], s.CPLSuffix) {
		add("the BOM %s and the placement list %s name different boards", boms[0], cpls[0])
	}
	for _, bomName := range boms {
		checkTable(bomName, files[bomName], s.BOMHeader, add, func(row map[string]string) {
			if strings.TrimSpace(row[s.BOMRefs]) == "" {
				add("%s: a row with no %s", bomName, s.BOMRefs)
			}
		})
	}
	for _, cplName := range cpls {
		checkTable(cplName, files[cplName], s.CPLHeader, add, func(row map[string]string) {
			ref := row[s.CPLRef]
			if strings.TrimSpace(ref) == "" {
				add("%s: a row with no %s", cplName, s.CPLRef)
			}
			for _, c := range []string{s.X, s.Y} {
				if !mmValue.MatchString(row[c]) {
					add("%s: %s: %s %q is not in mm, such as 1.5000mm", cplName, ref, c, row[c])
				}
			}
			if r, err := strconv.ParseFloat(row[s.Rotation], 64); err != nil || math.IsNaN(r) || r < 0 || r >= 360 {
				add("%s: %s: %s %q is not at least 0 and below 360", cplName, ref, s.Rotation, row[s.Rotation])
			}
			ok := false
			for _, l := range s.Layers {
				ok = ok || row[s.Layer] == l
			}
			if !ok {
				add("%s: %s: %s %q is not one of %s", cplName, ref, s.Layer, row[s.Layer], strings.Join(s.Layers, ", "))
			}
		})
	}
	sort.Strings(problems)
	return problems, nil
}

func read(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", f.Name, err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", f.Name, err)
	}
	return b, nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Verdict is the test's result: with want empty, an error if there are
// problems; with want set (a known-bad ZIP), an error unless one of the
// problems contains it.
func Verdict(problems []string, want string) error {
	if want == "" {
		if len(problems) > 0 {
			return fmt.Errorf("%d problems", len(problems))
		}
		return nil
	}
	for _, p := range problems {
		if strings.Contains(p, want) {
			return nil
		}
	}
	return fmt.Errorf("want a problem containing %q; got %d others", want, len(problems))
}

// checkTable checks a CSV's header and that it has rows, and calls each
// on every row.
func checkTable(name string, data []byte, header []string, add func(string, ...any), each func(map[string]string)) {
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		add("%s: %v", name, err)
		return
	}
	if len(rows) == 0 {
		add("%s: no header row", name)
		return
	}
	if !equal(rows[0], header) {
		add("%s: columns %q, want %q", name, rows[0], header)
		return
	}
	if len(rows) == 1 {
		add("%s: no rows", name)
	}
	for _, r := range rows[1:] {
		m := map[string]string{}
		for i, h := range header {
			m[h] = r[i]
		}
		each(m)
	}
}
