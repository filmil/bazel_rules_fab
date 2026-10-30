// SPDX-License-Identifier: Apache-2.0

// Package files_test checks that fab_rules picks kicad-druid's file for the
// vendor and stackup asked for.
package files_test

import (
	"flag"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var files = flag.String("files", "", "the fab_rules outputs, as runfiles paths")

// vendors maps the target name's vendor to the name kicad-druid's header uses.
var vendors = map[string]string{"generic": "Generic", "jlcpcb": "JLCPCB", "pcbway": "PCBWay"}

func TestFiles(t *testing.T) {
	all := append(strings.Fields(*files), flag.Args()...)
	if len(all) != 13 {
		t.Fatalf("%d files, want 13: %v", len(all), all)
	}
	for _, rpath := range all {
		p, err := runfiles.Rlocation(rpath)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		// <vendor>_<layers>L_<oz>oz.kicad_dru, or jlcpcb_default (4L-1oz).
		name := strings.TrimSuffix(path.Base(rpath), ".kicad_dru")
		parts := strings.SplitN(name, "_", 2)
		vendor, stackup := vendors[parts[0]], strings.Replace(parts[1], "_", "-", 1)
		if parts[1] == "default" {
			stackup = "4L-1oz"
		}
		if vendor == "" {
			t.Errorf("%s: unknown vendor", name)
			continue
		}
		if !strings.HasPrefix(s, "(version 1)") {
			t.Errorf("%s: not a KiCad design rules file", name)
		}
		if !strings.Contains(s, "for KiCad — "+vendor+":") {
			t.Errorf("%s: the header does not name %s", name, vendor)
		}
		if !strings.Contains(s, "This is the '"+stackup+"' variant") {
			t.Errorf("%s: not the %s variant", name, stackup)
		}
	}
}
