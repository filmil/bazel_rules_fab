// SPDX-License-Identifier: Apache-2.0

package swatch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Caps is what a fab house can make, read from its KiCad design rules
// (a kicad-druid .kicad_dru file). Lengths are in mm.
type Caps struct {
	// Vendor is the prefix of the rule names, such as "JLCPCB".
	Vendor string

	TrackWidth   float64 // the narrowest track
	TrackSpacing float64 // the smallest gap between tracks of two nets
	EdgeClear    float64 // the least distance from a track to the board edge

	ViaHole    float64 // the smallest via drill
	ViaAnnular float64 // the narrowest via ring
	// SmallVia: a via whose hole is below SmallViaHole needs a pad of at
	// least SmallViaPad (JLCPCB's extra-cost rule). Zero when the rules
	// have none.
	SmallViaHole, SmallViaPad float64

	PTHHole    float64 // the smallest plated through hole
	PTHAnnular float64 // the narrowest ring of a plated hole
	NPTHHole   float64 // the smallest non-plated hole

	PadSpacing float64 // the smallest gap between two SMD pads of two nets
	SMDPad     float64 // the smallest SMD pad side; zero when the rules have none

	TextHeight    float64 // the smallest silkscreen text
	TextThickness float64 // the thinnest silkscreen text stroke
	SilkClear     float64 // the least distance from an exposed pad to silkscreen
}

var (
	ruleRe       = regexp.MustCompile(`^\(rule "([^"]*)"`)
	conditionRe  = regexp.MustCompile(`^\(condition "(.*)"\)$`)
	constraintRe = regexp.MustCompile(`^\(constraint (\w+)(.*)\)$`)
	minRe        = regexp.MustCompile(`\(min ([0-9.]+)mm\)`)
	holeBelowRe  = regexp.MustCompile(`A\.Hole < ([0-9.]+)mm`)
	sizeAtLeast  = regexp.MustCompile(`A\.Size_X >= ([0-9.]+)mm`)
)

type rule struct {
	name, condition string
	// min holds each constraint's minimum by constraint type; assertion
	// holds an assertion's expression.
	min       map[string]float64
	assertion string
}

func parseRules(text string) ([]rule, error) {
	var rules []rule
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := ruleRe.FindStringSubmatch(line); m != nil {
			rules = append(rules, rule{name: m[1], min: map[string]float64{}})
			continue
		}
		if len(rules) == 0 {
			continue
		}
		r := &rules[len(rules)-1]
		if m := conditionRe.FindStringSubmatch(line); m != nil {
			r.condition = m[1]
			continue
		}
		if m := constraintRe.FindStringSubmatch(line); m != nil {
			if m[1] == "assertion" {
				r.assertion = m[2]
				continue
			}
			if mm := minRe.FindStringSubmatch(m[2]); mm != nil {
				v, err := strconv.ParseFloat(mm[1], 64)
				if err != nil {
					return nil, fmt.Errorf("line %d: %v", n+1, err)
				}
				r.min[m[1]] = v
			}
		}
	}
	return rules, nil
}

// ParseCaps reads a fab house's capabilities from its design rules.
func ParseCaps(text string) (Caps, error) {
	rules, err := parseRules(text)
	if err != nil {
		return Caps{}, err
	}
	var c Caps
	byName := map[string]rule{}
	for _, r := range rules {
		vendor, short, ok := strings.Cut(r.name, ": ")
		if !ok {
			continue
		}
		if c.Vendor == "" {
			c.Vendor = vendor
		}
		byName[short] = r
		if v, ok := r.min["via_diameter"]; ok {
			if m := holeBelowRe.FindStringSubmatch(r.condition); m != nil {
				c.SmallViaHole, _ = strconv.ParseFloat(m[1], 64)
				c.SmallViaPad = v
			}
		}
	}
	if c.Vendor == "" {
		return c, fmt.Errorf("no rule is named \"VENDOR: ...\"")
	}
	var missing []string
	need := func(dst *float64, constraint string, names ...string) {
		for _, n := range names {
			if r, ok := byName[n]; ok {
				if v, ok := r.min[constraint]; ok {
					*dst = v
					return
				}
			}
		}
		missing = append(missing, fmt.Sprintf("%q (%s)", names[0], constraint))
	}
	need(&c.TrackWidth, "track_width", "Trace Width", "Trace Width (Outer Layer)")
	need(&c.TrackSpacing, "clearance", "Trace Spacing", "Trace Spacing (Outer Layer)")
	need(&c.EdgeClear, "edge_clearance", "Trace to Board Edge")
	need(&c.ViaHole, "hole_size", "Via Hole Size")
	need(&c.ViaAnnular, "annular_width", "Via Annular Ring")
	need(&c.PTHHole, "hole_size", "PTH Hole Size")
	need(&c.PTHAnnular, "annular_width", "PTH Annular Ring")
	need(&c.NPTHHole, "hole_size", "NPTH Hole Size")
	need(&c.PadSpacing, "clearance", "Pad to Pad Clearance (Pad without Hole, Different Nets)")
	need(&c.TextHeight, "text_height", "Minimum Text Height")
	need(&c.TextThickness, "text_thickness", "Minimum Line Width")
	need(&c.SilkClear, "silk_clearance", "Pad to Silkscreen")
	if len(missing) > 0 {
		return c, fmt.Errorf("the %s rules lack %s", c.Vendor, strings.Join(missing, ", "))
	}
	if r, ok := byName["SMD Pad Size"]; ok {
		if m := sizeAtLeast.FindStringSubmatch(r.assertion); m != nil {
			c.SMDPad, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	return c, nil
}

// ViaPad is the smallest pad a via with this hole may have.
func (c Caps) ViaPad(hole float64) float64 {
	pad := hole + 2*c.ViaAnnular
	if c.SmallViaPad > 0 && hole < c.SmallViaHole && pad < c.SmallViaPad {
		pad = c.SmallViaPad
	}
	return round(pad)
}
