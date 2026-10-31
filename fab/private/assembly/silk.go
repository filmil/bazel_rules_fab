// SPDX-License-Identifier: Apache-2.0

package assembly

import (
	"fmt"
	"math"
	"strconv"
)

// OwnSilkMin is the least clearance the board's extra rule allows
// between a footprint's own silkscreen and its own pads. KiCad's library
// footprints draw their silkscreen 0.10 mm (an 0402) to 0.14 mm (a test
// point) from their pads, under the vendors' 0.15 mm; the fab clips
// silkscreen off pads.
const OwnSilkMin = 0.09

// OwnSilkRule is the rule the board's rules file adds after the vendor's
// rules, which it leaves as they are: a footprint's silkscreen against
// its own pads only. KiCad's Parent of a footprint's item is the
// footprint's reference, and of a board's item the board's UUID, so any
// other silkscreen, the generator's labels among it, keeps the vendor's
// clearance.
var OwnSilkRule = fmt.Sprintf(`
# hdlf_rules_fab's assembly swatch (issue #32): KiCad's own footprints,
# placed as they are, draw their silkscreen closer to their own pads than
# the vendor's pad-to-silkscreen clearance; the fab clips silkscreen off
# pads. This rule, after the vendor's, lets a footprint's silkscreen come
# %s mm from its own pads, and nothing else.
(rule "KiCad footprint silk on its own pads"
	(condition "A.Type == 'Pad' && B.Layer == 'F.SilkS' && A.Parent == B.Parent")
	(constraint silk_clearance (min %smm))
)
`, num(OwnSilkMin), num(OwnSilkMin))

// seg is a stroke: a line from a to b, w wide.
type seg struct {
	a, b pt
	w    float64
}

// shape is a pad's copper: a rectangle grown by r on every side (a round
// rectangle; a circle or an oval when the rectangle is a point or a line).
type shape struct {
	c      pt      // the center
	hw, hh float64 // the inner rectangle's half width and height
	r      float64
}

func padShape(p *Node) (shape, bool) {
	at, sz := p.Child("at"), p.Child("size")
	if at == nil || sz == nil {
		return shape{}, false
	}
	w, h := float(sz, 0), float(sz, 1)
	if rot := math.Mod(math.Abs(float(at, 2)), 180); rot > 45 && rot < 135 {
		w, h = h, w
	}
	c := pt{float(at, 0), float(at, 1)}
	m := math.Min(w, h)
	r := 0.0
	switch p.Arg(2) {
	case "circle", "oval":
		r = m / 2
	case "roundrect":
		ratio := 0.25
		if rr := p.Child("roundrect_rratio"); rr != nil {
			ratio = float(rr, 0)
		}
		r = ratio * m
	}
	return shape{c, w/2 - r, h/2 - r, r}, true
}

// distRect is the distance from q to the axis-aligned rectangle around c.
func distRect(q pt, s shape) float64 {
	dx := math.Max(math.Abs(q.x-s.c.x)-s.hw, 0)
	dy := math.Max(math.Abs(q.y-s.c.y)-s.hh, 0)
	return math.Hypot(dx, dy)
}

func distPointSeg(q, a, b pt) float64 {
	vx, vy := b.x-a.x, b.y-a.y
	l := vx*vx + vy*vy
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((q.x-a.x)*vx+(q.y-a.y)*vy)/l))
	}
	return math.Hypot(q.x-a.x-t*vx, q.y-a.y-t*vy)
}

func cross(o, a, b pt) float64 { return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x) }

func segsCross(a, b, c, d pt) bool {
	d1, d2 := cross(c, d, a), cross(c, d, b)
	d3, d4 := cross(a, b, c), cross(a, b, d)
	return ((d1 > 0) != (d2 > 0)) && ((d3 > 0) != (d4 > 0))
}

// distSegRect is the distance from the segment ab to the inner rectangle
// of s: zero when they meet.
func distSegRect(a, b pt, s shape) float64 {
	corners := []pt{
		{s.c.x - s.hw, s.c.y - s.hh}, {s.c.x + s.hw, s.c.y - s.hh},
		{s.c.x + s.hw, s.c.y + s.hh}, {s.c.x - s.hw, s.c.y + s.hh},
	}
	if distRect(a, s) == 0 || distRect(b, s) == 0 {
		return 0
	}
	d := math.Min(distRect(a, s), distRect(b, s))
	for i := range corners {
		c, e := corners[i], corners[(i+1)%4]
		if segsCross(a, b, c, e) {
			return 0
		}
		d = math.Min(d, distPointSeg(c, a, b))
	}
	return d
}

// clearance is the gap between a stroke and a pad's copper.
func (sg seg) clearance(s shape) float64 {
	return distSegRect(sg.a, sg.b, s) - s.r - sg.w/2
}

// strokes are a silkscreen item's outline, as straight strokes.
func strokes(it *Node) []seg {
	w := 0.0
	if st := it.Child("stroke"); st != nil {
		if wd := st.Child("width"); wd != nil {
			w = float(wd, 0)
		}
	}
	get := func(h string) pt { p, _ := xyOf(it, h); return p }
	switch it.Head() {
	case "fp_line":
		return []seg{{get("start"), get("end"), w}}
	case "fp_rect":
		a, b := get("start"), get("end")
		c, d := pt{b.x, a.y}, pt{a.x, b.y}
		return []seg{{a, c, w}, {c, b, w}, {b, d, w}, {d, a, w}}
	case "fp_poly":
		var ps []pt
		if pts := it.Child("pts"); pts != nil {
			for _, x := range pts.Children("xy") {
				ps = append(ps, pt{float(x, 0), float(x, 1)})
			}
		}
		var out []seg
		for i := range ps {
			out = append(out, seg{ps[i], ps[(i+1)%len(ps)], w})
		}
		return out
	case "fp_circle", "fp_arc":
		var c pt
		var r, a0, a1 float64
		if it.Head() == "fp_circle" {
			c = get("center")
			e := get("end")
			r, a0, a1 = math.Hypot(e.x-c.x, e.y-c.y), 0, 2*math.Pi
		} else {
			s, m, e := get("start"), get("mid"), get("end")
			var ok bool
			c, ok = circumcenter(s, m, e)
			if !ok {
				return []seg{{s, m, w}, {m, e, w}}
			}
			r = math.Hypot(s.x-c.x, s.y-c.y)
			as, am, ae := math.Atan2(s.y-c.y, s.x-c.x), math.Atan2(m.y-c.y, m.x-c.x), math.Atan2(e.y-c.y, e.x-c.x)
			a0, a1 = as, ae
			// Go the way that passes the middle point.
			if ccw(as, am) > ccw(as, ae) {
				a1 = as - (2*math.Pi - ccw(as, ae))
			} else {
				a1 = as + ccw(as, ae)
			}
		}
		const n = 180
		var out []seg
		prev := pt{c.x + r*math.Cos(a0), c.y + r*math.Sin(a0)}
		for i := 1; i <= n; i++ {
			t := a0 + (a1-a0)*float64(i)/n
			q := pt{c.x + r*math.Cos(t), c.y + r*math.Sin(t)}
			out = append(out, seg{prev, q, w})
			prev = q
		}
		return out
	}
	return nil
}

// ccw is the counter-clockwise angle from a to b, in [0, 2pi).
func ccw(a, b float64) float64 { return math.Mod(b-a+4*math.Pi, 2*math.Pi) }

func circumcenter(a, b, c pt) (pt, bool) {
	d := 2 * (a.x*(b.y-c.y) + b.x*(c.y-a.y) + c.x*(a.y-b.y))
	if math.Abs(d) < 1e-12 {
		return pt{}, false
	}
	a2, b2, c2 := a.x*a.x+a.y*a.y, b.x*b.x+b.y*b.y, c.x*c.x+c.y*c.y
	return pt{(a2*(b.y-c.y) + b2*(c.y-a.y) + c2*(a.y-b.y)) / d, (a2*(c.x-b.x) + b2*(a.x-c.x) + c2*(b.x-a.x)) / d}, true
}

// OwnSilkPairs counts the pairs of one of a footprint's F.SilkS items and
// one of its own pads that come closer than limit, as KiCad's DRC reports
// them, one violation per pair. It fails when a pair comes closer than
// OwnSilkMin, which the board's extra rule would not allow either.
func OwnSilkPairs(ref string, fp *Node, limit float64) (int, error) {
	var shapes []shape
	var names []string
	for _, p := range fp.Children("pad") {
		if !copper(p) {
			continue // a paste-only aperture: no copper for silkscreen to clear
		}
		if s, ok := padShape(p); ok {
			shapes = append(shapes, s)
			names = append(names, p.Arg(0))
		}
	}
	n := 0
	for _, it := range fp.List {
		l := it.Child("layer")
		if l == nil || l.Arg(0) != "F.SilkS" {
			continue
		}
		sg := strokes(it)
		if len(sg) == 0 {
			continue
		}
		for i, s := range shapes {
			d := math.Inf(1)
			for _, x := range sg {
				d = math.Min(d, x.clearance(s))
			}
			// A filled shape covers the pad if it holds the pad's center.
			if filled(it) && inside(sg, s.c) {
				d = 0
			}
			if d < limit-1e-6 {
				if d < OwnSilkMin-1e-6 {
					return 0, fmt.Errorf("%s: its %s comes %s mm from pad %s, under even %s mm", ref, it.Head(), strconv.FormatFloat(d, 'f', 3, 64), names[i], num(OwnSilkMin))
				}
				n++
			}
		}
	}
	return n, nil
}

// filled says whether a silkscreen shape is filled: (fill yes), (fill
// solid), or a polygon, which KiCad fills unless told not to.
func filled(it *Node) bool {
	f := it.Child("fill")
	if f == nil {
		return it.Head() == "fp_poly"
	}
	return f.Arg(0) == "yes" || f.Arg(0) == "solid"
}

// inside says whether q is inside the closed outline sg (even-odd rule).
func inside(sg []seg, q pt) bool {
	in := false
	for _, e := range sg {
		if (e.a.y > q.y) != (e.b.y > q.y) && q.x < e.a.x+(q.y-e.a.y)*(e.b.x-e.a.x)/(e.b.y-e.a.y) {
			in = !in
		}
	}
	return in
}
