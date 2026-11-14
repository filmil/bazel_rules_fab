// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Pt is a point in mm, Y up, as in Gerber.
type Pt struct{ X, Y float64 }

// Primitive kinds.
const (
	circle  = iota // P[0] the center, R the radius
	capsule        // a segment P[0]-P[1] widened by R, round ends
	polygon        // P, filled with the nonzero rule
)

// prim is one shape of an object. dark false clears what the object's
// earlier shapes set (an aperture's hole, a macro's exposure off).
type prim struct {
	kind int
	p    []Pt
	r    float64
	dark bool
}

// Object is one flash, stroke or region contour: its shapes, drawn in
// order, then put on the layer dark or clear (the layer's polarity).
type Object struct {
	Dark  bool
	prims []prim
}

// Layer is one parsed Gerber file.
type Layer struct {
	// Function is the file's .FileFunction attribute, such as
	// "Copper,L1,Top"; empty if it has none.
	Function string
	Objects  []Object
	// Path is the centre line of every stroke (D01), arcs as short
	// segments: the board outline, for a profile layer.
	Path [][2]Pt
}

// aperture is a flash's shape at the origin. convex is its outline for
// strokes by a non-circular aperture; round is the radius of a circular
// one (0 if it is not circular).
type aperture struct {
	prims  []prim
	round  float64
	convex []Pt
}

// ParseGerber reads an RS-274X file in absolute coordinates, as KiCad
// writes it. It refuses what it does not draw (incremental coordinates,
// single-quadrant arcs, step and repeat, aperture transformations), so a
// render is never quietly wrong.
func ParseGerber(data []byte) (*Layer, error) {
	p := &gerber{
		layer:  &Layer{},
		aps:    map[int]*aperture{},
		macros: map[string][]string{},
		scale:  1,
		dark:   true,
		interp: 1,
	}
	if err := p.run(string(data)); err != nil {
		return nil, err
	}
	return p.layer, nil
}

type gerber struct {
	layer      *Layer
	aps        map[int]*aperture
	macros     map[string][]string
	xInt, xDec int
	yDec       int
	fsSet      bool
	scale      float64 // to mm
	cur        *aperture
	curCode    int
	x, y       float64
	dark       bool
	interp     int // 1 linear, 2 clockwise, 3 counter-clockwise
	multiQuad  bool
	inRegion   bool
	contour    []Pt
	ended      bool
}

func (g *gerber) run(s string) error {
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '%':
			j := strings.IndexByte(s[i+1:], '%')
			if j < 0 {
				return fmt.Errorf("an extended command with no closing %%")
			}
			if err := g.extended(s[i+1 : i+1+j]); err != nil {
				return err
			}
			i += j + 2
		case c == '\n' || c == '\r' || c == ' ' || c == '\t':
			i++
		default:
			j := strings.IndexByte(s[i:], '*')
			if j < 0 {
				return fmt.Errorf("a word with no closing *: %.40q", s[i:])
			}
			if err := g.word(strings.TrimSpace(s[i : i+j])); err != nil {
				return err
			}
			i += j + 1
		}
		if g.ended {
			return nil
		}
	}
	if g.inRegion {
		return fmt.Errorf("a region (G36) with no end (G37)")
	}
	return nil
}

func clean(s string) string {
	return strings.NewReplacer("\n", "", "\r", "").Replace(s)
}

func (g *gerber) extended(body string) error {
	var words []string
	for _, w := range strings.Split(body, "*") {
		if w = clean(w); w != "" {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return nil
	}
	w := words[0]
	switch {
	case strings.HasPrefix(w, "FS"):
		return g.format(w)
	case w == "MOMM":
		g.scale = 1
	case w == "MOIN":
		g.scale = 25.4
	case strings.HasPrefix(w, "AM"):
		g.macros[w[2:]] = words[1:]
	case strings.HasPrefix(w, "AD"):
		return g.define(w)
	case w == "LPD":
		g.dark = true
	case w == "LPC":
		g.dark = false
	case w == "LMN" || w == "LR0" || strings.HasPrefix(w, "LR0.") && strings.Trim(w[3:], "0.") == "" ||
		w == "LS1" || strings.HasPrefix(w, "LS1.") && strings.Trim(w[4:], "0") == "":
		// Identity transformations.
	case strings.HasPrefix(w, "LM") || strings.HasPrefix(w, "LR") || strings.HasPrefix(w, "LS"):
		return fmt.Errorf("aperture transformation %s: not drawn", w)
	case strings.HasPrefix(w, "SR"):
		if w != "SR" && w != "SRX1Y1I0J0" && w != "SRX1Y1I0.0J0.0" {
			return fmt.Errorf("step and repeat %s: not drawn", w)
		}
	case strings.HasPrefix(w, "TF.FileFunction,"):
		g.layer.Function = strings.TrimPrefix(w, "TF.FileFunction,")
	case strings.HasPrefix(w, "TF") || strings.HasPrefix(w, "TA") || strings.HasPrefix(w, "TO") ||
		strings.HasPrefix(w, "TD") || strings.HasPrefix(w, "IP") || strings.HasPrefix(w, "LN") ||
		strings.HasPrefix(w, "IN"):
		// Attributes and names: no effect on the image.
	default:
		return fmt.Errorf("extended command %s: not known", w)
	}
	return nil
}

func (g *gerber) format(w string) error {
	// FSLAX46Y46: leading zeros omitted, absolute, 4 integer and 6
	// decimal digits.
	if len(w) != 10 || w[2] != 'L' || w[3] != 'A' || w[4] != 'X' || w[7] != 'Y' {
		return fmt.Errorf("format %s: only FSLAX<n><m>Y<n><m> is read", w)
	}
	g.xInt = int(w[5] - '0')
	g.xDec = int(w[6] - '0')
	g.yDec = int(w[9] - '0')
	g.fsSet = true
	return nil
}

func (g *gerber) define(w string) error {
	// ADD<code><template>[,<p>X<p>...]
	rest := w[3:]
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	code, err := strconv.Atoi(rest[:i])
	if err != nil || code < 10 {
		return fmt.Errorf("aperture %s: bad D code", w)
	}
	name, params := rest[i:], ""
	if k := strings.IndexByte(name, ','); k >= 0 {
		name, params = name[:k], name[k+1:]
	}
	var vals []float64
	if params != "" {
		for _, f := range strings.Split(params, "X") {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return fmt.Errorf("aperture %s: parameter %q: %v", w, f, err)
			}
			vals = append(vals, v)
		}
	}
	a, err := g.makeAperture(name, vals)
	if err != nil {
		return fmt.Errorf("aperture D%d: %v", code, err)
	}
	g.aps[code] = a
	return nil
}

func (g *gerber) makeAperture(name string, v []float64) (*aperture, error) {
	s := g.scale
	need := func(n int) error {
		if len(v) < n {
			return fmt.Errorf("%s needs %d parameters, has %d", name, n, len(v))
		}
		return nil
	}
	a := &aperture{}
	hole := func(i int) {
		if len(v) > i && v[i] > 0 {
			a.prims = append(a.prims, prim{kind: circle, p: []Pt{{}}, r: v[i] * s / 2})
		}
	}
	switch name {
	case "C":
		if err := need(1); err != nil {
			return nil, err
		}
		a.round = v[0] * s / 2
		a.prims = []prim{{kind: circle, p: []Pt{{}}, r: a.round, dark: true}}
		hole(1)
	case "R":
		if err := need(2); err != nil {
			return nil, err
		}
		w, h := v[0]*s/2, v[1]*s/2
		a.convex = []Pt{{-w, -h}, {w, -h}, {w, h}, {-w, h}}
		a.prims = []prim{{kind: polygon, p: a.convex, dark: true}}
		hole(2)
	case "O":
		if err := need(2); err != nil {
			return nil, err
		}
		w, h := v[0]*s, v[1]*s
		var p prim
		if w >= h {
			r := h / 2
			p = prim{kind: capsule, p: []Pt{{-w/2 + r, 0}, {w/2 - r, 0}}, r: r, dark: true}
		} else {
			r := w / 2
			p = prim{kind: capsule, p: []Pt{{0, -h/2 + r}, {0, h/2 - r}}, r: r, dark: true}
		}
		a.prims = []prim{p}
		a.convex = capsuleOutline(p)
		hole(2)
	case "P":
		if err := need(2); err != nil {
			return nil, err
		}
		rot := 0.0
		if len(v) > 2 {
			rot = v[2]
		}
		a.convex = regular(Pt{}, v[0]*s/2, int(v[1]), rot)
		a.prims = []prim{{kind: polygon, p: a.convex, dark: true}}
		hole(3)
	default:
		body, ok := g.macros[name]
		if !ok {
			return nil, fmt.Errorf("no aperture macro %q", name)
		}
		ps, err := evalMacro(body, v, s)
		if err != nil {
			return nil, fmt.Errorf("macro %s: %v", name, err)
		}
		a.prims = ps
	}
	return a, nil
}

func (g *gerber) word(w string) error {
	if w == "" {
		return nil
	}
	if strings.HasPrefix(w, "G04") {
		return nil
	}
	// Leading G codes; older files put them before coordinates.
	for strings.HasPrefix(w, "G") {
		i := 1
		for i < len(w) && w[i] >= '0' && w[i] <= '9' {
			i++
		}
		n, err := strconv.Atoi(w[1:i])
		if err != nil {
			return fmt.Errorf("word %q: bad G code", w)
		}
		w = w[i:]
		switch n {
		case 1, 2, 3:
			g.interp = n
		case 75:
			g.multiQuad = true
		case 74:
			return fmt.Errorf("single-quadrant arcs (G74): not drawn")
		case 36:
			g.inRegion = true
			g.contour = nil
		case 37:
			g.closeContour()
			g.inRegion = false
		case 54, 55:
		case 70:
			g.scale = 25.4
		case 71:
			g.scale = 1
		case 90:
		case 91:
			return fmt.Errorf("incremental coordinates (G91): not drawn")
		default:
			return fmt.Errorf("G%02d: not known", n)
		}
	}
	if w == "" {
		return nil
	}
	if w == "M02" || w == "M00" || w == "M01" {
		g.ended = true
		return nil
	}
	if w[0] == 'D' {
		code, err := strconv.Atoi(w[1:])
		if err != nil {
			return fmt.Errorf("word %q: bad D code", w)
		}
		if code >= 10 {
			a, ok := g.aps[code]
			if !ok {
				return fmt.Errorf("D%d: not defined", code)
			}
			g.cur, g.curCode = a, code
			return nil
		}
		return g.operate(code, g.x, g.y, 0, 0)
	}
	if !g.fsSet {
		return fmt.Errorf("coordinates before the format (FS)")
	}
	x, y, i, j := g.x, g.y, 0.0, 0.0
	op := -1
	for k := 0; k < len(w); {
		c := w[k]
		k++
		start := k
		for k < len(w) && (w[k] == '-' || w[k] == '+' || w[k] >= '0' && w[k] <= '9') {
			k++
		}
		num := w[start:k]
		if c == 'D' {
			n, err := strconv.Atoi(num)
			if err != nil {
				return fmt.Errorf("word %q: bad D code", w)
			}
			op = n
			continue
		}
		dec := g.xDec
		if c == 'Y' || c == 'J' {
			dec = g.yDec
		}
		v, err := g.coord(num, dec)
		if err != nil {
			return fmt.Errorf("word %q: %v", w, err)
		}
		switch c {
		case 'X':
			x = v
		case 'Y':
			y = v
		case 'I':
			i = v
		case 'J':
			j = v
		default:
			return fmt.Errorf("word %q: %c not known", w, c)
		}
	}
	if op < 0 {
		return fmt.Errorf("word %q: coordinates with no D01, D02 or D03", w)
	}
	return g.operate(op, x, y, i, j)
}

func (g *gerber) coord(s string, dec int) (float64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("coordinate %q: %v", s, err)
	}
	return float64(n) / math.Pow(10, float64(dec)) * g.scale, nil
}

func (g *gerber) closeContour() {
	if len(g.contour) >= 3 {
		g.layer.Objects = append(g.layer.Objects, Object{
			Dark:  g.dark,
			prims: []prim{{kind: polygon, p: g.contour, dark: true}},
		})
	}
	g.contour = nil
}

func (g *gerber) operate(op int, x, y, i, j float64) error {
	from := Pt{g.x, g.y}
	to := Pt{x, y}
	g.x, g.y = x, y
	switch op {
	case 2:
		if g.inRegion {
			g.closeContour()
			g.contour = []Pt{to}
		}
		return nil
	case 3:
		if g.inRegion {
			return fmt.Errorf("a flash (D03) inside a region")
		}
		if g.cur == nil {
			return fmt.Errorf("a flash (D03) with no aperture")
		}
		g.layer.Objects = append(g.layer.Objects, Object{Dark: g.dark, prims: moved(g.cur.prims, to)})
		return nil
	case 1:
	default:
		return fmt.Errorf("D%02d: not known", op)
	}
	path := []Pt{from, to}
	if g.interp != 1 {
		if !g.multiQuad {
			return fmt.Errorf("an arc with no G75 (multi-quadrant) before it")
		}
		path = arc(from, to, Pt{from.X + i, from.Y + j}, g.interp == 2)
	}
	if g.inRegion {
		if len(g.contour) == 0 {
			g.contour = []Pt{from}
		}
		g.contour = append(g.contour, path[1:]...)
		return nil
	}
	if g.cur == nil {
		return fmt.Errorf("a stroke (D01) with no aperture")
	}
	for k := 1; k < len(path); k++ {
		a, b := path[k-1], path[k]
		g.layer.Path = append(g.layer.Path, [2]Pt{a, b})
		var ps []prim
		switch {
		case g.cur.round > 0:
			ps = []prim{{kind: capsule, p: []Pt{a, b}, r: g.cur.round, dark: true}}
		case g.cur.convex != nil:
			pts := append(shift(g.cur.convex, a), shift(g.cur.convex, b)...)
			ps = []prim{{kind: polygon, p: hull(pts), dark: true}}
		default:
			return fmt.Errorf("a stroke with aperture D%d, which is not C, R, O or P", g.curCode)
		}
		g.layer.Objects = append(g.layer.Objects, Object{Dark: g.dark, prims: ps})
	}
	return nil
}

// arc returns the points of an arc from a to b around c, a and b
// included; a full circle when a is b.
func arc(a, b, c Pt, cw bool) []Pt {
	r := math.Hypot(a.X-c.X, a.Y-c.Y)
	t0 := math.Atan2(a.Y-c.Y, a.X-c.X)
	t1 := math.Atan2(b.Y-c.Y, b.X-c.X)
	sweep := t1 - t0
	if cw {
		for sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	} else {
		for sweep <= 0 {
			sweep += 2 * math.Pi
		}
	}
	if math.Abs(a.X-b.X) > 1e-9 || math.Abs(a.Y-b.Y) > 1e-9 {
		// Not a full circle: an exact 2π sweep is the zero one.
		if sweep == 2*math.Pi || sweep == -2*math.Pi {
			sweep = 0
		}
	}
	n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 90)))
	if n < 1 {
		n = 1
	}
	pts := []Pt{a}
	for k := 1; k < n; k++ {
		t := t0 + sweep*float64(k)/float64(n)
		pts = append(pts, Pt{c.X + r*math.Cos(t), c.Y + r*math.Sin(t)})
	}
	return append(pts, b)
}

func shift(ps []Pt, d Pt) []Pt {
	out := make([]Pt, len(ps))
	for i, p := range ps {
		out[i] = Pt{p.X + d.X, p.Y + d.Y}
	}
	return out
}

func moved(ps []prim, d Pt) []prim {
	out := make([]prim, len(ps))
	for i, p := range ps {
		out[i] = p
		out[i].p = shift(p.p, d)
	}
	return out
}

func rotate(p Pt, deg float64) Pt {
	if deg == 0 {
		return p
	}
	t := deg * math.Pi / 180
	s, c := math.Sin(t), math.Cos(t)
	return Pt{p.X*c - p.Y*s, p.X*s + p.Y*c}
}

func regular(c Pt, r float64, n int, rot float64) []Pt {
	if n < 3 {
		n = 3
	}
	pts := make([]Pt, n)
	for k := range pts {
		pts[k] = shift([]Pt{rotate(Pt{r, 0}, rot+360*float64(k)/float64(n))}, c)[0]
	}
	return pts
}

func capsuleOutline(p prim) []Pt {
	var pts []Pt
	for _, e := range p.p {
		pts = append(pts, regular(e, p.r, 72, 0)...)
	}
	return hull(pts)
}

// hull is the convex hull of ps, counter-clockwise (monotone chain).
func hull(ps []Pt) []Pt {
	pts := append([]Pt(nil), ps...)
	sortPts(pts)
	cross := func(o, a, b Pt) float64 { return (a.X-o.X)*(b.Y-o.Y) - (a.Y-o.Y)*(b.X-o.X) }
	var h []Pt
	for pass := 0; pass < 2; pass++ {
		start := len(h)
		for _, p := range pts {
			for len(h) >= start+2 && cross(h[len(h)-2], h[len(h)-1], p) <= 0 {
				h = h[:len(h)-1]
			}
			h = append(h, p)
		}
		h = h[:len(h)-1]
		for l, r := 0, len(pts)-1; l < r; l, r = l+1, r-1 {
			pts[l], pts[r] = pts[r], pts[l]
		}
	}
	return h
}

func sortPts(p []Pt) {
	// Insertion sort: hulls here have tens of points, and the order is
	// total, so the result does not depend on the sort's stability.
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && (p[j].X < p[j-1].X || p[j].X == p[j-1].X && p[j].Y < p[j-1].Y); j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}
