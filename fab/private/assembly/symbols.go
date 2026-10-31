// SPDX-License-Identifier: Apache-2.0

package assembly

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Library is a parsed .kicad_sym file.
type Library struct {
	Name    string // the library's nickname, such as "Device"
	symbols map[string]*Node
}

// ReadLibrary parses a .kicad_sym file's text.
func ReadLibrary(name, text string) (*Library, error) {
	root, err := Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	if root.Head() != "kicad_symbol_lib" {
		return nil, fmt.Errorf("%s: not a KiCad symbol library", name)
	}
	l := &Library{Name: name, symbols: map[string]*Node{}}
	for _, s := range root.Children("symbol") {
		l.symbols[s.Arg(0)] = s
	}
	return l, nil
}

// Symbol returns the symbol `name`, with any `extends` resolved: a derived
// symbol is its parent's body under its own name, with its own properties.
// The result is named "Lib:name", as a schematic's lib_symbols has it.
func (l *Library) Symbol(name string) (*Node, error) {
	s, err := l.resolve(name, 0)
	if err != nil {
		return nil, err
	}
	s = s.Copy()
	s.List[1] = Q(l.Name + ":" + name)
	return s, nil
}

func (l *Library) resolve(name string, depth int) (*Node, error) {
	s, ok := l.symbols[name]
	if !ok {
		return nil, fmt.Errorf("library %s has no symbol %q", l.Name, name)
	}
	ext := s.Child("extends")
	if ext == nil {
		return s, nil
	}
	if depth > 8 {
		return nil, fmt.Errorf("%s:%s: extends too deep", l.Name, name)
	}
	parent, err := l.resolve(ext.Arg(0), depth+1)
	if err != nil {
		return nil, err
	}
	out := parent.Copy()
	out.List[1] = Q(name)
	pname := parent.Arg(0)
	// The parent's units are named PARENT_<unit>_<style>.
	for _, u := range out.Children("symbol") {
		if rest, ok := strings.CutPrefix(u.Arg(0), pname+"_"); ok {
			u.List[1] = Q(name + "_" + rest)
		}
	}
	// The child's properties replace the parent's, or are added.
	for _, p := range s.Children("property") {
		replaced := false
		for i, q := range out.List {
			if q.Head() == "property" && q.Arg(0) == p.Arg(0) {
				out.List[i] = p.Copy()
				replaced = true
			}
		}
		if !replaced {
			out.List = append(out.List, p.Copy())
		}
	}
	return out, nil
}

// Pin is a symbol pin: its number, name, electrical type, and where its
// end (the point a wire or label connects to) is, in the symbol's
// coordinates (y up), and whether it is hidden.
type Pin struct {
	Number, Name, Type string
	X, Y               float64
	Hidden             bool
}

// Units returns, for each unit of the symbol (1, 2, ...), its pins, with
// the pins of unit 0 (common to all units) in every unit. The style
// (De Morgan) 1 is used.
func Units(sym *Node) (map[int][]Pin, error) {
	name := sym.Arg(0)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}
	byUnit := map[int][]Pin{}
	var common []Pin
	units := map[int]bool{}
	for _, u := range sym.Children("symbol") {
		rest, ok := strings.CutPrefix(u.Arg(0), name+"_")
		if !ok {
			return nil, fmt.Errorf("symbol %s: unit %q is not %s_<unit>_<style>", name, u.Arg(0), name)
		}
		parts := strings.Split(rest, "_")
		if len(parts) != 2 {
			return nil, fmt.Errorf("symbol %s: unit %q", name, u.Arg(0))
		}
		unit, err1 := strconv.Atoi(parts[0])
		style, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("symbol %s: unit %q", name, u.Arg(0))
		}
		if style > 1 {
			continue // a De Morgan body
		}
		if unit > 0 {
			units[unit] = true
		}
		for _, p := range u.Children("pin") {
			at := p.Child("at")
			x, _ := strconv.ParseFloat(at.Arg(0), 64)
			y, _ := strconv.ParseFloat(at.Arg(1), 64)
			pin := Pin{
				Number: p.Child("number").Arg(0),
				Name:   p.Child("name").Arg(0),
				Type:   p.Arg(0),
				X:      x, Y: y,
				Hidden: p.Child("hide") != nil && p.Child("hide").Arg(0) == "yes",
			}
			if unit == 0 {
				common = append(common, pin)
			} else {
				byUnit[unit] = append(byUnit[unit], pin)
			}
		}
	}
	if len(units) == 0 {
		units[1] = true
	}
	for u := range units {
		byUnit[u] = append(byUnit[u], common...)
		sort.Slice(byUnit[u], func(i, j int) bool { return pinLess(byUnit[u][i].Number, byUnit[u][j].Number) })
	}
	return byUnit, nil
}

func pinLess(a, b string) bool {
	x, e1 := strconv.Atoi(a)
	y, e2 := strconv.Atoi(b)
	if e1 == nil && e2 == nil {
		return x < y
	}
	return a < b
}
