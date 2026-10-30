// SPDX-License-Identifier: Apache-2.0

// Package fired_test checks a DRC report of kicad-druid's test board: the
// vendor's rules loaded, and they fire.
package fired_test

import (
	"encoding/json"
	"flag"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	report  = flag.String("report", "", "the DRC report, as a runfiles path")
	vendor  = flag.String("vendor", "", "the vendor's name in its rule names, such as JLCPCB")
	atLeast = flag.Int("at_least", 10, "how many of the vendor's rules must fire, at least")
)

// The rules every vendor has, which the test boards violate on purpose.
var core = []string{
	"Minimum Line Width",
	"NPTH Hole Size",
	"Pad to Trace",
	"PTH Annular Ring",
	"PTH Hole Size",
	"PTH to Trace",
}

func TestVendorRulesFire(t *testing.T) {
	p, err := runfiles.Rlocation(*report)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Violations []struct {
			Description string `json:"description"`
		} `json:"violations"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	// KiCad names the rule a violation breaks: "(rule 'JLCPCB: ...' ...".
	re := regexp.MustCompile(`rule '` + regexp.QuoteMeta(*vendor) + `: ([^']+)'`)
	fired := map[string]bool{}
	for _, v := range r.Violations {
		for _, m := range re.FindAllStringSubmatch(v.Description, -1) {
			fired[m[1]] = true
		}
	}
	var names []string
	for n := range fired {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) < *atLeast {
		t.Errorf("%d of %s's rules fire, want %d or more: %s", len(names), *vendor, *atLeast, strings.Join(names, "; "))
	}
	for _, c := range core {
		if !fired[c] {
			t.Errorf("%s: %s does not fire; fired: %s", *vendor, c, strings.Join(names, "; "))
		}
	}
}
