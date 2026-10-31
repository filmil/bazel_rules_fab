// SPDX-License-Identifier: Apache-2.0

// Package assembly writes the assembly swatch's schematic (issue #32):
// one rung per package, each part between two test points, so that a
// multimeter shows whether the assembly service soldered it.
//
// Each rung's part comes from KiCad's pinned symbol libraries; its two
// measured pins go, through net labels, to two test points; every other
// pin carries a no-connect flag; a net that holds a power input pin gets
// a PWR_FLAG, so ERC passes. The parts, their fields (LCSC, MPN and the
// like) and the pins come from rules_fab's vendor and parts tables
// (fab/private/assembly.bzl), passed in as JSON.
package assembly

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Rung is one part of the ladder.
type Rung struct {
	Ref       string            `json:"ref"`       // R1, U1, ...
	Symbol    string            `json:"symbol"`    // Lib:Name in KiCad's libraries
	Footprint string            `json:"footprint"` // Lib:Name
	Value     string            `json:"value"`
	Label     string            `json:"label"`   // the rung's name, such as "0402" or "SOIC-8"
	Reading   string            `json:"reading"` // what a meter reads, such as "1.00k 1%"
	Measure   []string          `json:"measure"` // the two measured pins, first the meter's +
	Fields    map[string]string `json:"fields"`  // Datasheet, LCSC, MPN, Manufacturer, ...
}

// Spec is the whole swatch.
type Spec struct {
	Project string `json:"project"` // the schematic's base name
	Title   string `json:"title"`
	Rungs   []Rung `json:"rungs"`
}

type sch struct {
	b       strings.Builder
	project string
	root    string
	n       int
	pwr     int
	tp      int
}

func (s *sch) uuid() string {
	s.n++
	h := sha256.Sum256([]byte(fmt.Sprintf("hdlf_rules_fab assembly %s %d", s.project, s.n)))
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func num(v float64) string { return strconv.FormatFloat(math.Round(v*1e4)/1e4, 'f', -1, 64) }

// grid rounds to KiCad's 1.27 mm connection grid.
func grid(v float64) float64 { return math.Round(v/1.27) * 1.27 }

func (s *sch) f(format string, a ...any) { fmt.Fprintf(&s.b, format, a...) }

func (s *sch) property(key, value string, x, y float64, hide bool) {
	h := ""
	if hide {
		h = "\n\t\t\t\t(hide yes)"
	}
	s.f("\t\t(property %q %q\n\t\t\t(at %s %s 0)\n\t\t\t(effects\n\t\t\t\t(font\n\t\t\t\t\t(size 1.27 1.27)\n\t\t\t\t)%s\n\t\t\t)\n\t\t)\n",
		key, value, num(x), num(y), h)
}

// Where a symbol goes: a part (BOM and board), a test point (board only:
// a bare pad, not an ordered part) or a flag (neither).
const (
	part = iota
	testPoint
	flag
)

// symbol places one unit of a symbol with its origin at (x, y).
func (s *sch) symbol(libID, ref, value string, unit int, x, y float64, kind int, props [][2]string, pins []Pin) {
	yn := map[bool]string{true: "yes", false: "no"}
	s.f("\t(symbol\n\t\t(lib_id %q)\n\t\t(at %s %s 0)\n\t\t(unit %d)\n\t\t(exclude_from_sim no)\n\t\t(in_bom %s)\n\t\t(on_board %s)\n\t\t(dnp no)\n\t\t(uuid %q)\n",
		libID, num(x), num(y), unit, yn[kind == part], yn[kind != flag], s.uuid())
	s.property("Reference", ref, x, y-3.81, kind == flag)
	s.property("Value", value, x, y+3.81, kind == flag)
	for _, p := range props {
		s.property(p[0], p[1], x, y, true)
	}
	for _, p := range pins {
		s.f("\t\t(pin %q\n\t\t\t(uuid %q)\n\t\t)\n", p.Number, s.uuid())
	}
	s.f("\t\t(instances\n\t\t\t(project %q\n\t\t\t\t(path \"/%s\"\n\t\t\t\t\t(reference %q)\n\t\t\t\t\t(unit %d)\n\t\t\t\t)\n\t\t\t)\n\t\t)\n\t)\n",
		s.project, s.root, ref, unit)
}

func (s *sch) label(name string, x, y float64) {
	s.f("\t(label %q\n\t\t(at %s %s 0)\n\t\t(effects\n\t\t\t(font\n\t\t\t\t(size 1.27 1.27)\n\t\t\t)\n\t\t\t(justify left bottom)\n\t\t)\n\t\t(uuid %q)\n\t)\n",
		name, num(x), num(y), s.uuid())
}

func (s *sch) noConnect(x, y float64) {
	s.f("\t(no_connect\n\t\t(at %s %s)\n\t\t(uuid %q)\n\t)\n", num(x), num(y), s.uuid())
}

func (s *sch) text(t string, x, y float64) {
	s.f("\t(text %q\n\t\t(exclude_from_sim no)\n\t\t(at %s %s 0)\n\t\t(effects\n\t\t\t(font\n\t\t\t\t(size 2 2)\n\t\t\t)\n\t\t\t(justify left bottom)\n\t\t)\n\t\t(uuid %q)\n\t)\n",
		t, num(x), num(y), s.uuid())
}

// Libraries maps a library nickname to its parsed file.
type Libraries map[string]*Library

func (ls Libraries) symbol(libID string) (*Node, error) {
	lib, name, ok := strings.Cut(libID, ":")
	if !ok {
		return nil, fmt.Errorf("symbol %q: want Lib:Name", libID)
	}
	l, ok := ls[lib]
	if !ok {
		return nil, fmt.Errorf("symbol %q: no library %s given", libID, lib)
	}
	return l.Symbol(name)
}

// Schematic returns the swatch's .kicad_sch.
func Schematic(spec Spec, libs Libraries) (string, error) {
	if spec.Project == "" || len(spec.Rungs) == 0 {
		return "", fmt.Errorf("the spec needs a project and rungs")
	}
	s := &sch{project: spec.Project}
	s.root = s.uuid()
	embedded := map[string]*Node{}
	embed := func(libID string) (*Node, error) {
		if n, ok := embedded[libID]; ok {
			return n, nil
		}
		n, err := libs.symbol(libID)
		if err != nil {
			return nil, err
		}
		embedded[libID] = n
		return n, nil
	}
	tpSym, err := embed("Connector:TestPoint")
	if err != nil {
		return "", err
	}
	tpPins, err := onePin(tpSym)
	if err != nil {
		return "", err
	}
	flagSym, err := embed("power:PWR_FLAG")
	if err != nil {
		return "", err
	}
	flagPins, err := onePin(flagSym)
	if err != nil {
		return "", err
	}
	refs := map[string]bool{}

	y := 30.0
	for _, r := range spec.Rungs {
		if refs[r.Ref] {
			return "", fmt.Errorf("reference %s appears twice", r.Ref)
		}
		refs[r.Ref] = true
		if len(r.Measure) != 2 || r.Measure[0] == r.Measure[1] {
			return "", fmt.Errorf("%s: measure needs two different pins, not %q", r.Ref, r.Measure)
		}
		sym, err := embed(r.Symbol)
		if err != nil {
			return "", fmt.Errorf("%s: %v", r.Ref, err)
		}
		units, err := Units(sym)
		if err != nil {
			return "", fmt.Errorf("%s: %v", r.Ref, err)
		}
		// The rung's height: the tallest unit's pin span.
		top, bottom := 0.0, 0.0
		for _, pins := range units {
			for _, p := range pins {
				top, bottom = math.Max(top, p.Y), math.Min(bottom, p.Y)
			}
		}
		oy := grid(y + top + 5.08)
		s.text(fmt.Sprintf("%s: %s, reads %s", r.Label, r.Value, r.Reading), 25.4, grid(y))
		props := [][2]string{{"Footprint", r.Footprint}}
		keys := make([]string, 0, len(r.Fields))
		for k := range r.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			props = append(props, [2]string{k, r.Fields[k]})
		}
		nets := map[string]string{r.Measure[0]: r.Ref + "_A", r.Measure[1]: r.Ref + "_B"}
		found := map[string]bool{}
		powerOn := map[string]bool{}
		var unitNums []int
		for u := range units {
			unitNums = append(unitNums, u)
		}
		sort.Ints(unitNums)
		x := 50.8
		for _, u := range unitNums {
			pins := units[u]
			s.symbol(r.Symbol, r.Ref, r.Value, u, x, oy, part, props, pins)
			// Each connection point once: stacked pins share one, and so
			// share the measured pin's net.
			type pt struct{ x, y float64 }
			netAt := map[pt]string{}
			for _, p := range pins {
				at := pt{grid(x + p.X), grid(oy - p.Y)}
				if net, ok := nets[p.Number]; ok {
					found[p.Number] = true
					if other, ok := netAt[at]; ok && other != net {
						return "", fmt.Errorf("%s: pins %s and %s are stacked", r.Ref, r.Measure[0], r.Measure[1])
					}
					if _, ok := netAt[at]; !ok {
						s.label(net, at.x, at.y)
					}
					netAt[at] = net
				}
			}
			done := map[pt]bool{}
			for _, p := range pins {
				at := pt{grid(x + p.X), grid(oy - p.Y)}
				if net, ok := netAt[at]; ok {
					if p.Type == "power_in" {
						powerOn[net] = true
					}
				} else if !done[at] {
					s.noConnect(at.x, at.y)
					done[at] = true
				}
			}
			x += 63.5
		}
		for _, m := range r.Measure {
			if !found[m] {
				return "", fmt.Errorf("%s: %s has no pin %q", r.Ref, r.Symbol, m)
			}
		}
		// The two test points, and a PWR_FLAG on a net with a power input.
		tx := x + 12.7
		for i, m := range r.Measure {
			s.tp++
			ref := fmt.Sprintf("TP%d", s.tp)
			ty := grid(oy - 5.08 + float64(i)*10.16)
			tpPin := tpPins[0]
			s.symbol("Connector:TestPoint", ref, nets[m], 1, tx, ty, testPoint,
				[][2]string{{"Footprint", "TestPoint:TestPoint_Pad_D1.5mm"}}, tpPins)
			s.label(nets[m], grid(tx+tpPin.X), grid(ty-tpPin.Y))
			if powerOn[nets[m]] {
				s.pwr++
				fx := tx + 25.4
				fp := flagPins[0]
				s.symbol("power:PWR_FLAG", fmt.Sprintf("#FLG%02d", s.pwr), "PWR_FLAG", 1, fx, ty, flag, [][2]string{{"Footprint", ""}}, flagPins)
				s.label(nets[m], grid(fx+fp.X), grid(ty-fp.Y))
			}
		}
		y = oy - bottom + 15.24
	}

	var out strings.Builder
	fmt.Fprintf(&out, "(kicad_sch\n\t(version 20250114)\n\t(generator \"hdlf_rules_fab_assembly\")\n\t(generator_version \"9.0\")\n\t(uuid %q)\n\t(paper \"A1\")\n", s.root)
	fmt.Fprintf(&out, "\t(title_block\n\t\t(title %q)\n\t)\n\t(lib_symbols\n", spec.Title)
	ids := make([]string, 0, len(embedded))
	for id := range embedded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out.WriteString("\t\t")
		Write(&out, embedded[id], 2)
		out.WriteString("\n")
	}
	out.WriteString("\t)\n")
	out.WriteString(s.b.String())
	out.WriteString("\t(sheet_instances\n\t\t(path \"/\"\n\t\t\t(page \"1\")\n\t\t)\n\t)\n\t(embedded_fonts no)\n)\n")
	return out.String(), nil
}

// onePin returns the pins of a one-unit symbol with one pin, such as a
// test point or a PWR_FLAG.
func onePin(sym *Node) ([]Pin, error) {
	units, err := Units(sym)
	if err != nil {
		return nil, err
	}
	if len(units) != 1 || len(units[1]) != 1 {
		return nil, fmt.Errorf("symbol %s: want one unit with one pin", sym.Arg(0))
	}
	return units[1], nil
}
