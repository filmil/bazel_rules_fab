// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Tool is one drill of a drill file.
type Tool struct {
	Code     int
	Diameter float64 // mm
	// Plated is "PTH", "NPTH" or "" when the file does not say.
	Plated string
	Holes  []Pt
	Slots  [][2]Pt
}

// Drill is one parsed drill file.
type Drill struct {
	Tools []*Tool // by code
}

// ParseExcellon reads a drill file as KiCad writes it: decimal
// coordinates, absolute, G85 slots, with the plating in KiCad's
// "; #@! TA.AperFunction" comments or the file's "TF.FileFunction".
func ParseExcellon(data []byte) (*Drill, error) {
	scale := 1.0
	byCode := map[int]*Tool{}
	plating := "" // of the next tool defined, from the comment before it
	fileFunc := ""
	header := false
	var cur *Tool
	x, y := 0.0, 0.0
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		where := func(err error) error { return fmt.Errorf("line %d %q: %v", n+1, line, err) }
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, ";"):
			c := strings.TrimSpace(strings.TrimPrefix(line, ";"))
			switch {
			case strings.HasPrefix(c, "#@! TF.FileFunction,"):
				fileFunc = c
			case strings.HasPrefix(c, "#@! TA.AperFunction,"):
				switch {
				case strings.Contains(c, ",NonPlated,"), strings.Contains(c, ",NPTH,"):
					plating = "NPTH"
				case strings.Contains(c, ",Plated,"), strings.Contains(c, ",PTH,"):
					plating = "PTH"
				}
			}
			continue
		case line == "M48":
			header = true
			continue
		case line == "%" || line == "M95":
			header = false
			continue
		case strings.HasPrefix(line, "METRIC"):
			scale = 1
			continue
		case strings.HasPrefix(line, "INCH"):
			scale = 25.4
			continue
		case strings.HasPrefix(line, "FMAT"), line == "G90", line == "G05", line == "M30",
			line == "M00", strings.HasPrefix(line, "VER"):
			continue
		case line == "M71":
			scale = 1
			continue
		case line == "M72":
			scale = 25.4
			continue
		case line == "G91" || line == "ICI,ON":
			return nil, where(fmt.Errorf("incremental coordinates: not drawn"))
		}
		if line[0] == 'T' {
			i := 1
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			code, err := strconv.Atoi(line[1:i])
			if err != nil {
				return nil, where(fmt.Errorf("bad tool number"))
			}
			rest := line[i:]
			if k := strings.IndexByte(rest, 'C'); k >= 0 {
				end := k + 1
				for end < len(rest) && (rest[end] >= '0' && rest[end] <= '9' || rest[end] == '.') {
					end++
				}
				d, err := strconv.ParseFloat(rest[k+1:end], 64)
				if err != nil {
					return nil, where(err)
				}
				switch t, ok := byCode[code]; {
				case !ok:
					byCode[code] = &Tool{Code: code, Diameter: d * scale, Plated: plating}
				case t.Diameter != d*scale:
					return nil, where(fmt.Errorf("tool T%d defined again with another diameter", code))
				}
				plating = ""
				if header {
					continue
				}
				// A definition in the body also selects the tool.
			} else if header {
				return nil, where(fmt.Errorf("a tool with no diameter"))
			}
			if code == 0 {
				cur = nil
				continue
			}
			t, ok := byCode[code]
			if !ok {
				return nil, where(fmt.Errorf("tool T%d: not defined", code))
			}
			cur = t
			continue
		}
		if line[0] != 'X' && line[0] != 'Y' {
			return nil, where(fmt.Errorf("not known"))
		}
		if cur == nil {
			return nil, where(fmt.Errorf("a hole with no tool"))
		}
		parts := strings.SplitN(line, "G85", 2)
		a, err := xy(parts[0], x, y, scale)
		if err != nil {
			return nil, where(err)
		}
		x, y = a.X, a.Y
		if len(parts) == 1 {
			cur.Holes = append(cur.Holes, a)
			continue
		}
		b, err := xy(parts[1], x, y, scale)
		if err != nil {
			return nil, where(err)
		}
		x, y = b.X, b.Y
		cur.Slots = append(cur.Slots, [2]Pt{a, b})
	}
	d := &Drill{}
	def := ""
	switch {
	case strings.Contains(fileFunc, "NonPlated"):
		def = "NPTH"
	case strings.Contains(fileFunc, ",Plated"):
		def = "PTH"
	}
	for _, t := range byCode {
		if t.Plated == "" {
			t.Plated = def
		}
		d.Tools = append(d.Tools, t)
	}
	sort.Slice(d.Tools, func(i, j int) bool { return d.Tools[i].Code < d.Tools[j].Code })
	return d, nil
}

// xy reads "X1.5Y-2" (either may be left out: it keeps its last value).
func xy(s string, x, y, scale float64) (Pt, error) {
	p := Pt{x, y}
	for s != "" {
		c := s[0]
		end := 1
		for end < len(s) && s[end] != 'X' && s[end] != 'Y' {
			end++
		}
		num := s[1:end]
		if !strings.Contains(num, ".") {
			return p, fmt.Errorf("coordinate %q has no decimal point: only decimal coordinates are read", num)
		}
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return p, err
		}
		switch c {
		case 'X':
			p.X = v * scale
		case 'Y':
			p.Y = v * scale
		default:
			return p, fmt.Errorf("%q: not a coordinate", s)
		}
		s = s[end:]
	}
	return p, nil
}
