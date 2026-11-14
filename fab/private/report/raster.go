// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

// SS is the supersampling: each output pixel is SS x SS samples.
const SS = 3

// Frame maps mm to samples: the board's box and a margin, Y flipped so
// that the image's top is the board's +Y side.
type Frame struct {
	MinX, MaxY float64 // mm, the image's top-left corner
	PxPerMM    int     // output pixels per mm
	W, H       int     // output pixels
}

// NewFrame fits a box in mm, with margin mm around it.
func NewFrame(minX, minY, maxX, maxY, margin float64, pxPerMM int) Frame {
	f := Frame{MinX: minX - margin, MaxY: maxY + margin, PxPerMM: pxPerMM}
	f.W = int(math.Ceil((maxX - minX + 2*margin) * float64(pxPerMM)))
	f.H = int(math.Ceil((maxY - minY + 2*margin) * float64(pxPerMM)))
	return f
}

// Mask is a layer at the sample resolution: true where it is dark.
type Mask struct {
	f    Frame
	w, h int
	bits []bool
}

// NewMask is an empty mask of the frame.
func NewMask(f Frame) *Mask {
	w, h := f.W*SS, f.H*SS
	return &Mask{f: f, w: w, h: h, bits: make([]bool, w*h)}
}

func (m *Mask) at(x, y int) bool { return m.bits[y*m.w+x] }

// scale is samples per mm.
func (m *Mask) scale() float64 { return float64(m.f.PxPerMM * SS) }

// sample returns the mm position of sample (i, j)'s centre.
func (m *Mask) sample(i, j int) Pt {
	s := m.scale()
	return Pt{m.f.MinX + (float64(i)+0.5)/s, m.f.MaxY - (float64(j)+0.5)/s}
}

// box returns the sample range [i0, i1) x [j0, j1) that covers the mm box.
func (m *Mask) box(minX, minY, maxX, maxY float64) (i0, j0, i1, j1 int) {
	s := m.scale()
	i0 = clamp(int(math.Floor((minX-m.f.MinX)*s)), 0, m.w)
	i1 = clamp(int(math.Ceil((maxX-m.f.MinX)*s))+1, 0, m.w)
	j0 = clamp(int(math.Floor((m.f.MaxY-maxY)*s)), 0, m.h)
	j1 = clamp(int(math.Ceil((m.f.MaxY-minY)*s))+1, 0, m.h)
	return
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func bounds(ps []prim) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, p := range ps {
		for _, q := range p.p {
			minX, maxX = math.Min(minX, q.X-p.r), math.Max(maxX, q.X+p.r)
			minY, maxY = math.Min(minY, q.Y-p.r), math.Max(maxY, q.Y+p.r)
		}
	}
	return
}

// Draw puts a layer's objects on the mask in order: each object's shapes
// make its image, which then sets (dark) or clears (clear polarity) the
// mask.
func (m *Mask) Draw(l *Layer) {
	for _, o := range l.Objects {
		m.drawObject(o)
	}
}

func (m *Mask) drawObject(o Object) {
	i0, j0, i1, j1 := m.box(bounds(o.prims))
	if i0 >= i1 || j0 >= j1 {
		return
	}
	w := i1 - i0
	local := make([]bool, w*(j1-j0))
	for _, p := range o.prims {
		m.fill(p, i0, j0, i1, j1, func(i, j int) { local[(j-j0)*w+(i-i0)] = p.dark })
	}
	for j := j0; j < j1; j++ {
		for i := i0; i < i1; i++ {
			if local[(j-j0)*w+(i-i0)] {
				m.bits[j*m.w+i] = o.Dark
			}
		}
	}
}

// fill calls set for every sample of the box [i0,i1)x[j0,j1) inside p.
func (m *Mask) fill(p prim, i0, j0, i1, j1 int, set func(i, j int)) {
	switch p.kind {
	case circle, capsule:
		a := p.p[0]
		b := a
		if p.kind == capsule {
			b = p.p[1]
		}
		pi0, pj0, pi1, pj1 := m.box(math.Min(a.X, b.X)-p.r, math.Min(a.Y, b.Y)-p.r,
			math.Max(a.X, b.X)+p.r, math.Max(a.Y, b.Y)+p.r)
		r2 := p.r * p.r
		for j := max(j0, pj0); j < min(j1, pj1); j++ {
			for i := max(i0, pi0); i < min(i1, pi1); i++ {
				if segDist2(m.sample(i, j), a, b) <= r2 {
					set(i, j)
				}
			}
		}
	case polygon:
		m.scan([][]Pt{p.p}, false, i0, j0, i1, j1, set)
	}
}

// scan fills polygons by rows, with the nonzero rule, or the even-odd
// rule when evenOdd.
func (m *Mask) scan(polys [][]Pt, evenOdd bool, i0, j0, i1, j1 int, set func(i, j int)) {
	type cross struct {
		x   float64
		dir int
	}
	s := m.scale()
	var xs []cross
	for j := j0; j < j1; j++ {
		y := m.sample(0, j).Y
		xs = xs[:0]
		for _, poly := range polys {
			n := len(poly)
			for k := 0; k < n; k++ {
				a, b := poly[k], poly[(k+1)%n]
				switch {
				case a.Y <= y && b.Y > y:
					xs = append(xs, cross{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), 1})
				case b.Y <= y && a.Y > y:
					xs = append(xs, cross{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), -1})
				}
			}
		}
		sort.Slice(xs, func(p, q int) bool {
			if xs[p].x != xs[q].x {
				return xs[p].x < xs[q].x
			}
			return xs[p].dir < xs[q].dir
		})
		wind := 0
		for k := 0; k+1 < len(xs); k++ {
			if evenOdd {
				wind ^= 1
			} else {
				wind += xs[k].dir
			}
			if wind == 0 {
				continue
			}
			// Samples whose centre lies in [xs[k].x, xs[k+1].x).
			from := int(math.Ceil((xs[k].x-m.f.MinX)*s - 0.5))
			to := int(math.Ceil((xs[k+1].x-m.f.MinX)*s - 0.5))
			for i := max(from, i0); i < min(to, i1); i++ {
				set(i, j)
			}
		}
	}
}

func segDist2(p, a, b Pt) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / l
		t = math.Max(0, math.Min(1, t))
	}
	ex, ey := p.X-(a.X+t*dx), p.Y-(a.Y+t*dy)
	return ex*ex + ey*ey
}

// DrawDrill puts a drill file's holes and slots on the mask.
func (m *Mask) DrawDrill(d *Drill) {
	for _, t := range d.Tools {
		r := t.Diameter / 2
		for _, h := range t.Holes {
			m.drawObject(Object{Dark: true, prims: []prim{{kind: circle, p: []Pt{h}, r: r, dark: true}}})
		}
		for _, s := range t.Slots {
			m.drawObject(Object{Dark: true, prims: []prim{{kind: capsule, p: []Pt{s[0], s[1]}, r: r, dark: true}}})
		}
	}
}

// FillLoops fills closed loops with the even-odd rule: a loop inside
// another is a cut-out.
func (m *Mask) FillLoops(loops [][]Pt) {
	m.scan(loops, true, 0, 0, m.w, m.h, func(i, j int) { m.bits[j*m.w+i] = true })
}

// Loops joins an outline's segments into closed loops. It returns the
// loops, and an error if some segments do not close.
func Loops(segs [][2]Pt) ([][]Pt, error) {
	type key struct{ x, y int64 }
	k := func(p Pt) key { return key{int64(math.Round(p.X * 1e4)), int64(math.Round(p.Y * 1e4))} }
	at := map[key][]int{}
	var kept [][2]Pt
	for _, s := range segs {
		if k(s[0]) != k(s[1]) { // a zero-length stroke joins nothing
			kept = append(kept, s)
		}
	}
	segs = kept
	for i, s := range segs {
		at[k(s[0])] = append(at[k(s[0])], i)
		at[k(s[1])] = append(at[k(s[1])], i)
	}
	used := make([]bool, len(segs))
	var loops [][]Pt
	open := 0
	for i := range segs {
		if used[i] {
			continue
		}
		used[i] = true
		start := segs[i][0]
		loop := []Pt{segs[i][0]}
		end := segs[i][1]
		closed := false
		for {
			if k(end) == k(start) {
				closed = true
				break
			}
			next := -1
			for _, c := range at[k(end)] {
				if !used[c] {
					next = c
					break
				}
			}
			if next < 0 {
				break
			}
			used[next] = true
			loop = append(loop, end)
			if k(segs[next][0]) == k(end) {
				end = segs[next][1]
			} else {
				end = segs[next][0]
			}
		}
		if closed && len(loop) >= 3 {
			loops = append(loops, loop)
		} else {
			open++
		}
	}
	if open > 0 {
		return loops, fmt.Errorf("%d open chain(s) in the outline", open)
	}
	return loops, nil
}

// Paint makes an image from masks: each output pixel is the mean of its
// samples' colours, which color picks from the masks at a sample.
// mirror flips it left to right (a board's bottom, seen from below).
func Paint(f Frame, mirror bool, color func(at func(*Mask) bool) color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, f.W, f.H))
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			var r, g, b, a int
			for sy := 0; sy < SS; sy++ {
				for sx := 0; sx < SS; sx++ {
					i, j := x*SS+sx, y*SS+sy
					c := color(func(m *Mask) bool { return m != nil && m.at(i, j) })
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
				}
			}
			n := SS * SS
			ox := x
			if mirror {
				ox = f.W - 1 - x
			}
			img.SetNRGBA(ox, y, nrgba(r/n, g/n, b/n, a/n))
		}
	}
	return img
}

func nrgba(r, g, b, a int) color.NRGBA { return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)} }
