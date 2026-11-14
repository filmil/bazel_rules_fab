// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// evalMacro draws an aperture macro's primitives with the AD parameters
// v ($1, $2, ...), scale to mm. It reads the primitives KiCad and most
// CAD programs write: 1 circle, 4 outline, 5 polygon, 7 thermal, 20
// vector line and 21 centre line; 0 is a comment.
func evalMacro(body []string, v []float64, scale float64) ([]prim, error) {
	vars := map[int]float64{}
	for i, x := range v {
		vars[i+1] = x
	}
	var out []prim
	for _, w := range body {
		w = strings.TrimSpace(w)
		if w == "" || strings.HasPrefix(w, "0") && (len(w) == 1 || w[1] == ' ' || w[1] == ',') {
			continue
		}
		if strings.HasPrefix(w, "$") {
			eq := strings.IndexByte(w, '=')
			if eq < 0 {
				return nil, fmt.Errorf("%q: neither a primitive nor $n=...", w)
			}
			n, err := strconv.Atoi(w[1:eq])
			if err != nil {
				return nil, fmt.Errorf("%q: bad variable", w)
			}
			x, err := evalExpr(w[eq+1:], vars)
			if err != nil {
				return nil, fmt.Errorf("%q: %v", w, err)
			}
			vars[n] = x
			continue
		}
		fields := strings.Split(w, ",")
		code, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("%q: bad primitive code", w)
		}
		var a []float64
		for _, f := range fields[1:] {
			x, err := evalExpr(f, vars)
			if err != nil {
				return nil, fmt.Errorf("%q: %v", w, err)
			}
			a = append(a, x)
		}
		need := func(n int) error {
			if len(a) < n {
				return fmt.Errorf("%q: primitive %d needs %d values, has %d", w, code, n, len(a))
			}
			return nil
		}
		mm := func(i int) float64 { return a[i] * scale }
		rot := func(i int) float64 {
			if len(a) > i {
				return a[i]
			}
			return 0
		}
		dark := func() (bool, error) {
			switch a[0] {
			case 0:
				return false, nil
			case 1:
				return true, nil
			}
			return false, fmt.Errorf("%q: exposure %v: only 0 and 1 are drawn", w, a[0])
		}
		switch code {
		case 1: // exposure, diameter, x, y[, rotation]
			if err := need(4); err != nil {
				return nil, err
			}
			d, err := dark()
			if err != nil {
				return nil, err
			}
			c := rotate(Pt{mm(2), mm(3)}, rot(4))
			out = append(out, prim{kind: circle, p: []Pt{c}, r: mm(1) / 2, dark: d})
		case 4: // exposure, n, x0, y0, ..., xn, yn, rotation
			if err := need(2); err != nil {
				return nil, err
			}
			n := int(a[1])
			if n < 1 {
				return nil, fmt.Errorf("%q: an outline of %d vertices", w, n)
			}
			if err := need(2 + 2*(n+1) + 1); err != nil {
				return nil, err
			}
			d, err := dark()
			if err != nil {
				return nil, err
			}
			r := a[2+2*(n+1)]
			var pts []Pt
			for k := 0; k <= n; k++ {
				pts = append(pts, rotate(Pt{mm(2 + 2*k), mm(3 + 2*k)}, r))
			}
			out = append(out, prim{kind: polygon, p: pts, dark: d})
		case 5: // exposure, vertices, x, y, diameter, rotation
			if err := need(5); err != nil {
				return nil, err
			}
			d, err := dark()
			if err != nil {
				return nil, err
			}
			// The vertices, the first on the centre's +X side, then the
			// whole turned about the origin.
			pts := regular(Pt{mm(2), mm(3)}, mm(4)/2, int(a[1]), 0)
			for k := range pts {
				pts[k] = rotate(pts[k], rot(5))
			}
			out = append(out, prim{kind: polygon, p: pts, dark: d})
		case 7: // x, y, outer, inner, gap, rotation
			if err := need(5); err != nil {
				return nil, err
			}
			// Four dark ring segments; the centre and the gaps leave the
			// macro's other shapes as they are.
			for _, seg := range thermal(mm(2)/2, mm(3)/2, mm(4)/2) {
				for k := range seg {
					seg[k] = rotate(Pt{seg[k].X + mm(0), seg[k].Y + mm(1)}, rot(5))
				}
				out = append(out, prim{kind: polygon, p: seg, dark: true})
			}
		case 20: // exposure, width, x1, y1, x2, y2, rotation
			if err := need(6); err != nil {
				return nil, err
			}
			d, err := dark()
			if err != nil {
				return nil, err
			}
			s, e := Pt{mm(2), mm(3)}, Pt{mm(4), mm(5)}
			out = append(out, prim{kind: polygon, p: band(s, e, mm(1)/2, rot(6)), dark: d})
		case 21: // exposure, width, height, x, y, rotation
			if err := need(5); err != nil {
				return nil, err
			}
			d, err := dark()
			if err != nil {
				return nil, err
			}
			w, h, x, y := mm(1)/2, mm(2)/2, mm(3), mm(4)
			box := []Pt{{x - w, y - h}, {x + w, y - h}, {x + w, y + h}, {x - w, y + h}}
			for k := range box {
				box[k] = rotate(box[k], rot(5))
			}
			out = append(out, prim{kind: polygon, p: box, dark: d})
		default:
			return nil, fmt.Errorf("%q: primitive %d: not drawn", w, code)
		}
	}
	return out, nil
}

// thermal returns a thermal's four ring segments, centred on the origin:
// the ring of radii ro and ri without the bands |x| < g and |y| < g.
func thermal(ro, ri, g float64) [][]Pt {
	var segs [][]Pt
	if g >= ro || ri >= ro {
		return nil
	}
	// One segment, in the first quadrant: the outer arc from the X gap to
	// the Y gap, then back along the inner arc (or the gaps' corner).
	ao := math.Asin(g / ro)
	var first []Pt
	const n = 45
	for k := 0; k <= n; k++ {
		t := ao + (math.Pi/2-2*ao)*float64(k)/n
		first = append(first, Pt{ro * math.Cos(t), ro * math.Sin(t)})
	}
	if g < ri {
		ai := math.Asin(g / ri)
		for k := n; k >= 0; k-- {
			t := ai + (math.Pi/2-2*ai)*float64(k)/n
			first = append(first, Pt{ri * math.Cos(t), ri * math.Sin(t)})
		}
	} else {
		first = append(first, Pt{g, g})
	}
	for q := 0; q < 4; q++ {
		seg := make([]Pt, len(first))
		for k, p := range first {
			seg[k] = rotate(p, 90*float64(q))
		}
		segs = append(segs, seg)
	}
	return segs
}

// band is the rectangle of half-width hw along s-e, square ends, turned
// by rot degrees about the origin.
func band(s, e Pt, hw, rot float64) []Pt {
	dx, dy := e.X-s.X, e.Y-s.Y
	l := math.Hypot(dx, dy)
	nx, ny := 0.0, hw
	if l > 0 {
		nx, ny = -dy/l*hw, dx/l*hw
	}
	pts := []Pt{{s.X + nx, s.Y + ny}, {s.X - nx, s.Y - ny}, {e.X - nx, e.Y - ny}, {e.X + nx, e.Y + ny}}
	for k := range pts {
		pts[k] = rotate(pts[k], rot)
	}
	return pts
}

// evalExpr evaluates a macro expression: numbers, $n, + - x / and
// parentheses, x and / before + and -.
func evalExpr(s string, vars map[int]float64) (float64, error) {
	e := &expr{s: strings.ReplaceAll(strings.TrimSpace(s), " ", ""), vars: vars}
	v, err := e.sum()
	if err != nil {
		return 0, err
	}
	if e.i != len(e.s) {
		return 0, fmt.Errorf("expression %q: unexpected %q", s, e.s[e.i:])
	}
	return v, nil
}

type expr struct {
	s    string
	i    int
	vars map[int]float64
}

func (e *expr) peek() byte {
	if e.i < len(e.s) {
		return e.s[e.i]
	}
	return 0
}

func (e *expr) sum() (float64, error) {
	v, err := e.product()
	if err != nil {
		return 0, err
	}
	for c := e.peek(); c == '+' || c == '-'; c = e.peek() {
		e.i++
		w, err := e.product()
		if err != nil {
			return 0, err
		}
		if c == '+' {
			v += w
		} else {
			v -= w
		}
	}
	return v, nil
}

func (e *expr) product() (float64, error) {
	v, err := e.unary()
	if err != nil {
		return 0, err
	}
	for c := e.peek(); c == 'x' || c == 'X' || c == '/'; c = e.peek() {
		e.i++
		w, err := e.unary()
		if err != nil {
			return 0, err
		}
		if c == '/' {
			v /= w
		} else {
			v *= w
		}
	}
	return v, nil
}

func (e *expr) unary() (float64, error) {
	switch e.peek() {
	case '-':
		e.i++
		v, err := e.unary()
		return -v, err
	case '+':
		e.i++
		return e.unary()
	case '(':
		e.i++
		v, err := e.sum()
		if err != nil {
			return 0, err
		}
		if e.peek() != ')' {
			return 0, fmt.Errorf("expression %q: no closing )", e.s)
		}
		e.i++
		return v, nil
	case '$':
		e.i++
		j := e.i
		for e.i < len(e.s) && e.s[e.i] >= '0' && e.s[e.i] <= '9' {
			e.i++
		}
		n, err := strconv.Atoi(e.s[j:e.i])
		if err != nil {
			return 0, fmt.Errorf("expression %q: bad variable", e.s)
		}
		// An unset variable is 0, as the Gerber specification says.
		return e.vars[n], nil
	}
	j := e.i
	for e.i < len(e.s) && (e.s[e.i] >= '0' && e.s[e.i] <= '9' || e.s[e.i] == '.') {
		e.i++
	}
	if j == e.i {
		return 0, fmt.Errorf("expression %q: a number expected at %d", e.s, j)
	}
	return strconv.ParseFloat(e.s[j:e.i], 64)
}
