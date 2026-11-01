// SPDX-License-Identifier: Apache-2.0

package assembly

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/swatch"
)

// The board, mm.
const (
	// Where the card's top left corner sits on KiCad's page.
	originX, originY = 20.0, 20.0
	// A rung's silkscreen text: tall enough for every vendor's minimum.
	textSize  = 1.0
	textThick = 0.15
	// Each test point's track.
	trackWidth = 0.25
	// The test point: KiCad's TestPoint_Pad_D1.5mm. Its pad's radius and
	// its courtyard's.
	testPointID  = "TestPoint:TestPoint_Pad_D1.5mm"
	tpPadR       = 0.75
	tpCourtyardR = 1.25
	// Room between courtyards, and between a track and a pad it passes.
	gap       = 0.3
	clearance = 0.2
	// How far a test point's courtyard stays from its part's, and how far
	// out of the part's courtyard the ring runs that joins a net's other
	// pads to its test point's track.
	tpGap   = 0.6
	ringGap = 0.3
	// The card's margin, and the room between rungs.
	margin  = 3.0
	cellGap = 1.5
)

// Footprints maps a footprint's Lib:Name to its parsed .kicad_mod.
type Footprints map[string]*Node

// ReadFootprint parses a .kicad_mod.
func ReadFootprint(text string) (*Node, error) {
	n, err := Parse(text)
	if err != nil {
		return nil, err
	}
	if n.Head() != "footprint" {
		return nil, fmt.Errorf("not a footprint: (%s ...)", n.Head())
	}
	return n, nil
}

type pt struct{ x, y float64 }

type box struct{ x0, y0, x1, y1 float64 }

func emptyBox() box { return box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)} }

func (b *box) add(x, y float64) {
	b.x0, b.y0 = math.Min(b.x0, x), math.Min(b.y0, y)
	b.x1, b.y1 = math.Max(b.x1, x), math.Max(b.y1, y)
}

func (b *box) addBox(o box) { b.add(o.x0, o.y0); b.add(o.x1, o.y1) }

func (b box) ok() bool { return b.x0 <= b.x1 && b.y0 <= b.y1 }

func float(n *Node, i int) float64 {
	v, _ := strconv.ParseFloat(n.Arg(i), 64)
	return v
}

func xyOf(n *Node, head string) (pt, bool) {
	c := n.Child(head)
	if c == nil {
		return pt{}, false
	}
	return pt{float(c, 0), float(c, 1)}, true
}

// courtyard is the bounding box of a footprint's F.CrtYd items.
func courtyard(fp *Node) box {
	b := emptyBox()
	for _, c := range fp.List {
		if l := c.Child("layer"); l == nil || l.Arg(0) != "F.CrtYd" {
			continue
		}
		switch c.Head() {
		case "fp_rect", "fp_line":
			for _, h := range []string{"start", "end"} {
				if p, ok := xyOf(c, h); ok {
					b.add(p.x, p.y)
				}
			}
		case "fp_arc":
			for _, h := range []string{"start", "mid", "end"} {
				if p, ok := xyOf(c, h); ok {
					b.add(p.x, p.y)
				}
			}
		case "fp_circle":
			ctr, ok1 := xyOf(c, "center")
			end, ok2 := xyOf(c, "end")
			if ok1 && ok2 {
				r := math.Hypot(end.x-ctr.x, end.y-ctr.y)
				b.add(ctr.x-r, ctr.y-r)
				b.add(ctr.x+r, ctr.y+r)
			}
		case "fp_poly":
			if pts := c.Child("pts"); pts != nil {
				for _, p := range pts.Children("xy") {
					b.add(float(p, 0), float(p, 1))
				}
			}
		}
	}
	return b
}

// pad is a footprint pad: its number, center and size, rotated.
type pad struct {
	num  string
	at   pt
	w, h float64
}

// pads are a footprint's copper pads; paste-only apertures, such as an
// exposed pad's, are left out.
func pads(fp *Node) []pad {
	var out []pad
	for _, p := range fp.Children("pad") {
		at := p.Child("at")
		sz := p.Child("size")
		if at == nil || sz == nil || !copper(p) {
			continue
		}
		w, h := float(sz, 0), float(sz, 1)
		if r := math.Mod(math.Abs(float(at, 2)), 180); r > 45 && r < 135 {
			w, h = h, w
		}
		out = append(out, pad{p.Arg(0), pt{float(at, 0), float(at, 1)}, w, h})
	}
	return out
}

// copper says whether a pad has copper: a layer *.Cu or ?.Cu.
func copper(p *Node) bool {
	l := p.Child("layers")
	if l == nil {
		return false
	}
	for _, c := range l.List[1:] {
		if strings.HasSuffix(c.Atom, ".Cu") {
			return true
		}
	}
	return false
}

// exit is where a measured pad's test point goes: straight out of the
// footprint from the pad, past the courtyard.
type exit struct {
	pin   string
	pad   pt
	dir   pt      // a unit vector along x or y
	edge  float64 // from the pad to the courtyard, along dir
	dist  float64 // from the pad to the test point
	label string  // "+" or "-"
}

// ring is where the exit crosses the ring around the courtyard.
func (e exit) ring() pt {
	d := e.edge + ringGap
	return pt{e.pad.x + e.dir.x*d, e.pad.y + e.dir.y*d}
}

func (e exit) tp() pt { return pt{e.pad.x + e.dir.x*e.dist, e.pad.y + e.dir.y*e.dist} }

// lateral is the pad's coordinate across its exit direction.
func (e exit) lateral() float64 {
	if e.dir.x != 0 {
		return e.pad.y
	}
	return e.pad.x
}

// exits places the two test points of a rung's part, in footprint
// coordinates.
func exits(r Rung, fp *Node) ([]exit, error) {
	cy := courtyard(fp)
	if !cy.ok() {
		return nil, fmt.Errorf("%s: footprint %s has no courtyard", r.Ref, r.Footprint)
	}
	all := pads(fp)
	pb := emptyBox()
	for _, p := range all {
		pb.add(p.at.x, p.at.y)
	}
	c := pt{(pb.x0 + pb.x1) / 2, (pb.y0 + pb.y1) / 2}
	hx, hy := math.Max((pb.x1-pb.x0)/2, 1e-6), math.Max((pb.y1-pb.y0)/2, 1e-6)
	var out []exit
	for i, m := range r.Measure {
		var p *pad
		for j := range all {
			if all[j].num == m {
				p = &all[j]
				break
			}
		}
		if p == nil {
			return nil, fmt.Errorf("%s: footprint %s has no pad %q", r.Ref, r.Footprint, m)
		}
		dx, dy := p.at.x-c.x, p.at.y-c.y
		var d pt
		var edge float64
		switch {
		case math.Abs(dx)/hx >= math.Abs(dy)/hy && dx > 0:
			d, edge = pt{1, 0}, cy.x1-p.at.x
		case math.Abs(dx)/hx >= math.Abs(dy)/hy && dx < 0:
			d, edge = pt{-1, 0}, p.at.x-cy.x0
		case dy > 0:
			d, edge = pt{0, 1}, cy.y1-p.at.y
		case dy < 0:
			d, edge = pt{0, -1}, p.at.y-cy.y0
		default:
			return nil, fmt.Errorf("%s: pad %s is at the middle of %s, with no way out", r.Ref, m, r.Footprint)
		}
		label := "+"
		if i == 1 {
			label = "-"
		}
		out = append(out, exit{m, p.at, d, edge, edge + tpCourtyardR + tpGap, label})
	}
	// Two exits on one side: the second test point goes one test point
	// further out, and its track passes beside the first test point's pad.
	a, b := out[0], out[1]
	if a.dir == b.dir {
		lat := math.Abs(a.lateral() - b.lateral())
		if lat < 2*tpCourtyardR+gap {
			if lat < tpPadR+trackWidth/2+clearance {
				return nil, fmt.Errorf("%s: pads %s and %s of %s leave on one side %s mm apart, too close for two test points",
					r.Ref, a.pin, b.pin, r.Footprint, num(lat))
			}
			out[1].dist = math.Max(a.dist, b.dist) + 2*tpCourtyardR + gap
		}
	}
	// Tracks must not cross: two exits on opposite sides never do; on
	// adjacent sides, they would only if a test point lay on the other's
	// path, which the courtyard margin rules out.
	return out, nil
}

// around returns the way, as corner points, from a to b along the ring
// around the box cy (grown by ringGap), the shorter way that passes none
// of the points in avoid; a, b and avoid lie on the ring.
func around(cy box, a, b pt, avoid []pt) ([]pt, error) {
	r := box{cy.x0 - ringGap, cy.y0 - ringGap, cy.x1 + ringGap, cy.y1 + ringGap}
	w, h := r.x1-r.x0, r.y1-r.y0
	per := 2 * (w + h)
	// Clockwise, from the top left corner.
	at := func(p pt) float64 {
		const eps = 1e-6
		switch {
		case math.Abs(p.y-r.y0) < eps:
			return p.x - r.x0
		case math.Abs(p.x-r.x1) < eps:
			return w + p.y - r.y0
		case math.Abs(p.y-r.y1) < eps:
			return w + h + r.x1 - p.x
		default:
			return 2*w + h + r.y1 - p.y
		}
	}
	corners := []struct {
		s float64
		p pt
	}{{0, pt{r.x0, r.y0}}, {w, pt{r.x1, r.y0}}, {w + h, pt{r.x1, r.y1}}, {2*w + h, pt{r.x0, r.y1}}}
	sa, sb := at(a), at(b)
	// cw is the distance clockwise from s to t.
	cw := func(s, t float64) float64 { return math.Mod(t-s+per, per) }
	way := func(clockwise bool) ([]pt, float64, bool) {
		from, to := sa, sb
		if !clockwise {
			from, to = sb, sa
		}
		span := cw(from, to)
		for _, v := range avoid {
			if d := cw(from, at(v)); d > 1e-6 && d < span-1e-6 {
				return nil, 0, false
			}
		}
		var pts []pt
		for _, c := range corners {
			if d := cw(from, c.s); d > 1e-6 && d < span-1e-6 {
				pts = append(pts, c.p)
			}
		}
		sort.Slice(pts, func(i, j int) bool { return cw(from, at(pts[i])) < cw(from, at(pts[j])) })
		if !clockwise {
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		return pts, span, true
	}
	p1, l1, ok1 := way(true)
	p2, l2, ok2 := way(false)
	switch {
	case ok1 && (!ok2 || l1 <= l2):
		return p1, nil
	case ok2:
		return p2, nil
	}
	return nil, fmt.Errorf("both ways round are blocked")
}

// cell is a rung laid out around its footprint's origin.
type cell struct {
	r      Rung
	fp     *Node
	exits  []exit
	bounds box // everything, footprint coordinates
	title  string
	titleY float64 // footprint coordinates
	labels []textAt
}

type textAt struct {
	s    string
	x, y float64
}

// textWidth is a generous width of a line of KiCad's stroke font.
func textWidth(s string) float64 { return float64(len(s)) * textSize * 0.9 }

func layout(r Rung, fp *Node) (*cell, error) {
	ex, err := exits(r, fp)
	if err != nil {
		return nil, err
	}
	c := &cell{r: r, fp: fp, exits: ex}
	b := courtyard(fp)
	// The ring that joins stacked pads may run round the courtyard.
	ring := ringGap + trackWidth/2
	b.add(b.x0-ring, b.y0-ring)
	b.add(b.x1+ring, b.y1+ring)
	for _, p := range pads(fp) {
		b.add(p.at.x-p.w/2, p.at.y-p.h/2)
		b.add(p.at.x+p.w/2, p.at.y+p.h/2)
	}
	for i, e := range ex {
		tp := e.tp()
		b.add(tp.x-tpCourtyardR, tp.y-tpCourtyardR)
		b.add(tp.x+tpCourtyardR, tp.y+tpCourtyardR)
		// The polarity mark beside its test point, across the exit, on
		// the side away from the other test point.
		// Compare along this test point's own lateral axis, also when
		// the other one leaves on a perpendicular side.
		other := ex[1-i].tp()
		side := 1.0
		if (e.dir.x != 0 && tp.y < other.y) || (e.dir.x == 0 && tp.x < other.x) {
			side = -1
		}
		off := tpCourtyardR + textSize/2 + gap
		var at pt
		if e.dir.x != 0 {
			at = pt{tp.x, tp.y + side*off}
		} else {
			at = pt{tp.x + side*off, tp.y}
		}
		c.labels = append(c.labels, textAt{e.label, at.x, at.y})
		b.add(at.x-textSize/2, at.y-textSize/2-textThick)
		b.add(at.x+textSize/2, at.y+textSize/2+textThick)
	}
	c.title = r.Label + "  " + r.Silk
	c.titleY = b.y0 - gap - textSize/2 - textThick
	w := textWidth(c.title)
	mid := (b.x0 + b.x1) / 2
	b.add(mid-w/2, c.titleY-textSize/2-textThick)
	b.add(mid+w/2, c.titleY)
	c.bounds = b
	return c, nil
}

type pcb struct {
	b       strings.Builder
	project string
	n       int
	nets    []string
}

func (p *pcb) uuid() string {
	p.n++
	h := sha256.Sum256([]byte(fmt.Sprintf("hdlf_rules_fab assembly board %s %d", p.project, p.n)))
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func (p *pcb) net(name string) int {
	for i, n := range p.nets {
		if n == name {
			return i + 1
		}
	}
	p.nets = append(p.nets, name)
	return len(p.nets)
}

func xy(x, y float64) string { return num(originX+x) + " " + num(originY+y) }

func (p *pcb) text(s string, x, y, size float64) {
	fmt.Fprintf(&p.b, "\t(gr_text %q\n\t\t(at %s 0)\n\t\t(layer \"F.SilkS\")\n\t\t(uuid %q)\n\t\t(effects\n\t\t\t(font\n\t\t\t\t(size %s %s)\n\t\t\t\t(thickness %s)\n\t\t\t)\n\t\t)\n\t)\n",
		s, xy(x, y), p.uuid(), num(size), num(size), num(textThick))
}

func (p *pcb) track(a, b pt, net int) {
	fmt.Fprintf(&p.b, "\t(segment\n\t\t(start %s)\n\t\t(end %s)\n\t\t(width %s)\n\t\t(layer \"F.Cu\")\n\t\t(net %d)\n\t\t(uuid %q)\n\t)\n",
		xy(a.x, a.y), xy(b.x, b.y), num(trackWidth), net, p.uuid())
}

// fresh gives every uuid in n a new one: two copies of a footprint must
// not share their items' UUIDs.
func (p *pcb) fresh(n *Node) {
	if n.Head() == "uuid" && len(n.List) == 2 {
		n.List[1] = Q(p.uuid())
		return
	}
	for _, c := range n.List {
		if c.IsList {
			p.fresh(c)
		}
	}
}

func property(key, value string, hide bool) *Node {
	k := []*Node{Q(key), Q(value), L("at", A("0"), A("0"), A("0")), L("layer", Q("F.Fab"))}
	if hide {
		k = append(k, L("hide", A("yes")))
	}
	k = append(k, L("uuid", Q("")), L("effects", L("font", L("size", A("1"), A("1")), L("thickness", A("0.15")))))
	return L("property", k...)
}

// place writes one footprint at (x, y): the library's footprint as it is,
// with the board's reference, value, fields, symbol path and pad nets.
func (p *pcb) place(libID string, lib *Node, x, y float64, ref, value, symUUID string, fields map[string]string, padNets map[string]int) {
	fp := lib.Copy()
	keep := []*Node{A("footprint"), Q(libID)}
	var props, rest []*Node
	for _, c := range fp.List[2:] {
		switch c.Head() {
		case "version", "generator", "generator_version":
			// The library file's own header.
		case "layer":
			keep = append(keep, c, L("uuid", Q("")), L("at", A(num(originX+x)), A(num(originY+y))))
		case "property":
			switch c.Arg(0) {
			case "Reference":
				c.List[2] = Q(ref)
				// The rung's own labels name the parts; the references
				// stay on the fabrication layer only.
				if c.Child("hide") == nil {
					c.List = append(c.List, L("hide", A("yes")))
				}
			case "Value":
				c.List[2] = Q(value)
			case "Datasheet":
				if d, ok := fields["Datasheet"]; ok {
					c.List[2] = Q(d)
				}
			}
			props = append(props, c)
		case "pad":
			if n, ok := padNets[c.Arg(0)]; ok {
				netNode := L("net", A(strconv.Itoa(n)), Q(p.nets[n-1]))
				var k []*Node
				placed := false
				for _, e := range c.List {
					if e.Head() == "uuid" && !placed {
						k = append(k, netNode)
						placed = true
					}
					k = append(k, e)
				}
				if !placed {
					k = append(k, netNode)
				}
				c.List = k
			}
			rest = append(rest, c)
		default:
			rest = append(rest, c)
		}
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "Datasheet" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		props = append(props, property(k, fields[k], true))
	}
	fp.List = append(keep, props...)
	fp.List = append(fp.List, L("path", Q("/"+symUUID)))
	fp.List = append(fp.List, rest...)
	p.fresh(fp)
	p.b.WriteString("\t")
	Write(&p.b, fp, 1)
	p.b.WriteString("\n")
}

// joinCenter joins the pads of each net that no test point measures
// and that holds more than one pad, such as an op-amp's V- pin and its
// exposed pad, which KiCad's symbol stacks: a straight track from the
// edge pad, inward, into the center pad. It fails on any other shape.
func (p *pcb) joinCenter(c *cell, ox, oy float64, padNets map[string]int) error {
	measured := map[int]bool{}
	for _, e := range c.exits {
		measured[padNets[e.pin]] = true
	}
	all := pads(c.fp)
	byNet := map[int][]pad{}
	var order []int
	for _, q := range all {
		n, ok := padNets[q.num]
		if !ok || measured[n] {
			continue
		}
		if byNet[n] == nil {
			order = append(order, n)
		}
		byNet[n] = append(byNet[n], q)
	}
	for _, n := range order {
		qs := byNet[n]
		if len(qs) < 2 {
			continue
		}
		// The center pad covers the footprint's origin.
		var center *pad
		var edges []pad
		for i, q := range qs {
			if math.Abs(q.at.x) <= q.w/2 && math.Abs(q.at.y) <= q.h/2 {
				center = &qs[i]
			} else {
				edges = append(edges, q)
			}
		}
		if center == nil || len(edges) != 1 {
			return fmt.Errorf("%s: pads %s share the net %s; only an edge pad and the center pad can be joined", c.r.Ref, padNums(qs), p.nets[n-1])
		}
		e := edges[0]
		// Inward along the edge pad's axis, to the center pad's middle
		// line, which must cross the center pad.
		end := pt{center.at.x, e.at.y}
		if math.Abs(e.at.x-center.at.x) < math.Abs(e.at.y-center.at.y) {
			end = pt{e.at.x, center.at.y}
		}
		if math.Abs(end.x-center.at.x) > center.w/2 || math.Abs(end.y-center.at.y) > center.h/2 {
			return fmt.Errorf("%s: pad %s does not face the center pad %s", c.r.Ref, e.num, center.num)
		}
		// The track must clear every other pad.
		for _, q := range all {
			if q.at == e.at || q.at == center.at {
				continue
			}
			sh := shape{q.at, q.w / 2, q.h / 2, 0}
			if d := (seg{e.at, end, trackWidth}).clearance(sh); d < clearance {
				return fmt.Errorf("%s: the track from pad %s to the center pad %s comes %s mm from pad %s", c.r.Ref, e.num, center.num, num(d), q.num)
			}
		}
		p.track(pt{ox + e.at.x, oy + e.at.y}, pt{ox + end.x, oy + end.y}, n)
	}
	return nil
}

func padNums(qs []pad) string {
	var s []string
	for _, q := range qs {
		s = append(s, q.num)
	}
	return strings.Join(s, ", ")
}

// joinStacked routes each pad that shares a measured pad's net to that
// pad's track.
func (p *pcb) joinStacked(c *cell, ox, oy float64, padNets map[string]int) error {
	cy := courtyard(c.fp)
	all := pads(c.fp)
	pb := emptyBox()
	for _, q := range all {
		pb.add(q.at.x, q.at.y)
	}
	ctr := pt{(pb.x0 + pb.x1) / 2, (pb.y0 + pb.y1) / 2}
	hx, hy := math.Max((pb.x1-pb.x0)/2, 1e-6), math.Max((pb.y1-pb.y0)/2, 1e-6)
	for i, e := range c.exits {
		net := padNets[e.pin]
		other := c.exits[1-i]
		for _, q := range all {
			if padNets[q.num] != net || q.at == e.pad {
				continue
			}
			dx, dy := q.at.x-ctr.x, q.at.y-ctr.y
			var d pt
			var edge float64
			switch {
			case math.Abs(dx)/hx >= math.Abs(dy)/hy && dx > 0:
				d, edge = pt{1, 0}, cy.x1-q.at.x
			case math.Abs(dx)/hx >= math.Abs(dy)/hy && dx < 0:
				d, edge = pt{-1, 0}, q.at.x-cy.x0
			case dy > 0:
				d, edge = pt{0, 1}, cy.y1-q.at.y
			case dy < 0:
				d, edge = pt{0, -1}, q.at.y-cy.y0
			default:
				return fmt.Errorf("%s: pad %s, on the net of pad %s, is at the middle of %s, with no way out", c.r.Ref, q.num, e.pin, c.r.Footprint)
			}
			start := pt{q.at.x + d.x*(edge+ringGap), q.at.y + d.y*(edge+ringGap)}
			way, err := around(cy, start, e.ring(), []pt{other.ring()})
			if err != nil {
				return fmt.Errorf("%s: pad %s, on the net of pad %s: %v", c.r.Ref, q.num, e.pin, err)
			}
			pts := append([]pt{q.at, start}, way...)
			pts = append(pts, e.ring())
			for k := 0; k+1 < len(pts); k++ {
				p.track(pt{ox + pts[k].x, oy + pts[k].y}, pt{ox + pts[k+1].x, oy + pts[k+1].y}, net)
			}
		}
	}
	return nil
}

// BoardOptions are the vendor's numbers the board needs, and its
// known-bad variants.
type BoardOptions struct {
	// SilkClear is the vendor's pad-to-silkscreen clearance, mm.
	SilkClear float64
	// Bad is a known-bad variant, whose DRC must find silk_overlap under
	// the vendor's own clearance, though the board's extra rule is there:
	// "label_on_pad" puts the first rung's label on its first measured
	// pad; "silk_near_pad" draws a silkscreen line BadSilkGap from that
	// pad, closer than the vendor allows but farther than the extra rule
	// allows a footprint's own silkscreen.
	Bad string
}

// Bads are the known-bad variants of BoardOptions.Bad.
var Bads = []string{"label_on_pad", "silk_near_pad"}

// BadSilkGap is how far silk_near_pad's line comes from the pad, mm.
const BadSilkGap = 0.12

// Board returns the swatch's .kicad_pcb: the card, and each rung's part
// with its two test points and their tracks, labelled on the silkscreen.
// sch gives each reference's symbol UUID and pin nets in the schematic,
// so that the board and the schematic pair up. It also returns how many
// pairs of a footprint's own silkscreen item and its own pad come closer
// than the vendor's clearance: the violations the board's extra rule
// (OwnSilkRule) answers for, as DRC without it reports them.
func Board(spec Spec, fps Footprints, sch *placed, o BoardOptions) (string, int, error) {
	if o.SilkClear <= 0 {
		return "", 0, fmt.Errorf("no pad-to-silkscreen clearance given")
	}
	if o.Bad != "" && o.Bad != Bads[0] && o.Bad != Bads[1] {
		return "", 0, fmt.Errorf("bad = %q: want one of %q", o.Bad, Bads)
	}
	tpFP, ok := fps[testPointID]
	if !ok {
		return "", 0, fmt.Errorf("no footprint %s given", testPointID)
	}
	var cells []*cell
	for _, r := range spec.Rungs {
		fp, ok := fps[r.Footprint]
		if !ok {
			return "", 0, fmt.Errorf("%s: no footprint %s given", r.Ref, r.Footprint)
		}
		c, err := layout(r, fp)
		if err != nil {
			return "", 0, err
		}
		cells = append(cells, c)
	}

	p := &pcb{project: spec.Project}
	p.b.WriteString(swatch.Outline(originX, originY, p.uuid))
	p.text(spec.Title, swatch.CardW/2, 3.2, 1.2)

	// The rungs, smallest first, left to right in rows; the rows spread
	// down the card below the title, each centered.
	const top = 5.5
	var rows [][]*cell
	var widths, heights []float64
	for _, c := range cells {
		w, h := c.bounds.x1-c.bounds.x0, c.bounds.y1-c.bounds.y0
		n := len(rows)
		if n == 0 || widths[n-1]+cellGap+w > swatch.CardW-2*margin {
			rows = append(rows, nil)
			widths = append(widths, -cellGap)
			heights = append(heights, 0)
			n++
		}
		rows[n-1] = append(rows[n-1], c)
		widths[n-1] += cellGap + w
		heights[n-1] = math.Max(heights[n-1], h)
	}
	total := 0.0
	for _, h := range heights {
		total += h
	}
	room := swatch.CardH - margin - top - total
	if room < 0 {
		return "", 0, fmt.Errorf("the rungs do not fit on the card: their rows are %s mm tall, with room for %s", num(total), num(swatch.CardH-margin-top))
	}
	vgap := room / float64(len(rows)+1)
	type spot struct {
		c    *cell
		x, y float64
	}
	var spots []spot
	y := top + vgap
	for i, row := range rows {
		x := (swatch.CardW - widths[i]) / 2
		for _, c := range row {
			h := c.bounds.y1 - c.bounds.y0
			// Each cell sits at the middle of its row's height.
			spots = append(spots, spot{c, x, y + (heights[i]-h)/2})
			x += c.bounds.x1 - c.bounds.x0 + cellGap
		}
		y += heights[i] + vgap
	}
	tp, pairs, first := 0, 0, true
	for _, sp := range spots {
		c, x, y := sp.c, sp.x, sp.y
		ox, oy := x-c.bounds.x0, y-c.bounds.y0
		r := c.r
		padNets := map[string]int{}
		pins := sch.pinNets[r.Ref]
		nums := make([]string, 0, len(pins))
		for n := range pins {
			nums = append(nums, n)
		}
		sort.Slice(nums, func(i, j int) bool { return pinLess(nums[i], nums[j]) })
		for _, n := range nums {
			padNets[n] = p.net(pins[n])
		}
		for _, e := range c.exits {
			if _, ok := padNets[e.pin]; !ok {
				return "", 0, fmt.Errorf("%s: measured pin %s has no net in the schematic", r.Ref, e.pin)
			}
		}
		uuid, ok := sch.symbols[r.Ref]
		if !ok {
			return "", 0, fmt.Errorf("%s: no symbol in the schematic", r.Ref)
		}
		p.place(r.Footprint, c.fp, ox, oy, r.Ref, r.Value, uuid, r.Fields, padNets)
		n, err := OwnSilkPairs(r.Ref, c.fp, o.SilkClear)
		if err != nil {
			return "", 0, err
		}
		pairs += n
		for _, e := range c.exits {
			tp++
			ref := fmt.Sprintf("TP%d", tp)
			net := padNets[e.pin]
			at := e.tp()
			tu, ok := sch.symbols[ref]
			if !ok {
				return "", 0, fmt.Errorf("%s: no symbol in the schematic", ref)
			}
			// The value is the net's label, as in the schematic.
			p.place(testPointID, tpFP, ox+at.x, oy+at.y, ref, strings.TrimPrefix(p.nets[net-1], "/"), tu, nil, map[string]int{"1": net})
			n, err := OwnSilkPairs(ref, tpFP, o.SilkClear)
			if err != nil {
				return "", 0, err
			}
			pairs += n
			ring := e.ring()
			p.track(pt{ox + e.pad.x, oy + e.pad.y}, pt{ox + ring.x, oy + ring.y}, net)
			p.track(pt{ox + ring.x, oy + ring.y}, pt{ox + at.x, oy + at.y}, net)
		}
		// Other pads on a measured net (stacked pins, such as an MCU's
		// VSS pins): out of the part, then round it to the measured
		// pad's track.
		if err := p.joinCenter(c, ox, oy, padNets); err != nil {
			return "", 0, err
		}
		if err := p.joinStacked(c, ox, oy, padNets); err != nil {
			return "", 0, err
		}
		for _, l := range c.labels {
			p.text(l.s, ox+l.x, oy+l.y, textSize)
		}
		tx, ty := ox+(c.bounds.x0+c.bounds.x1)/2, oy+c.titleY
		if o.Bad == "label_on_pad" && first {
			tx, ty = ox+c.exits[0].pad.x, oy+c.exits[0].pad.y
		}
		if o.Bad == "silk_near_pad" && first {
			// A line along the pad's top edge, BadSilkGap above it.
			for _, q := range pads(c.fp) {
				if q.num == c.exits[0].pin {
					const w = textThick
					y := oy + q.at.y - q.h/2 - BadSilkGap - w/2
					fmt.Fprintf(&p.b, "\t(gr_line\n\t\t(start %s)\n\t\t(end %s)\n\t\t(stroke\n\t\t\t(width %s)\n\t\t\t(type solid)\n\t\t)\n\t\t(layer \"F.SilkS\")\n\t\t(uuid %q)\n\t)\n",
						xy(ox+q.at.x-q.w/2, y), xy(ox+q.at.x+q.w/2, y), num(w), p.uuid())
					break
				}
			}
		}
		first = false
		p.text(c.title, tx, ty, textSize)
	}

	var out strings.Builder
	out.WriteString("(kicad_pcb\n\t(version 20241229)\n\t(generator \"hdlf_rules_fab_assembly\")\n\t(generator_version \"9.0\")\n")
	out.WriteString("\t(general\n\t\t(thickness 1.6)\n\t\t(legacy_teardrops no)\n\t)\n\t(paper \"A4\")\n")
	out.WriteString(swatch.LayerTable(2))
	out.WriteString("\t(setup\n\t\t(pad_to_mask_clearance 0)\n\t\t(allow_soldermask_bridges_in_footprints no)\n\t\t(tenting front back)\n\t)\n")
	out.WriteString("\t(net 0 \"\")\n")
	for i, n := range p.nets {
		fmt.Fprintf(&out, "\t(net %d %q)\n", i+1, n)
	}
	out.WriteString(p.b.String())
	out.WriteString("\t(embedded_fonts no)\n)\n")
	return out.String(), pairs, nil
}

// Project returns the swatch's .kicad_pro, for its schematic and its
// board: the board's own minimums are zero, so the vendor's rules alone
// decide, and every DRC check stays on, the library footprint checks
// among them, as the footprints are KiCad's own.
func Project(name string) string { return swatch.ProjectIgnoring(name, nil) }
