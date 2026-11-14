// SPDX-License-Identifier: Apache-2.0

// Command report is fab_report's tool: it writes a fab package's report
// and renders into a directory.
//
//	report --zip PKG.zip --vendor jlcpcb [--bare] [--px_per_mm 20] --out DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/filmil/bazel_rules_fab/fab/private/report"
)

func run(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	zipPath := fs.String("zip", "", "the fab package's ZIP")
	vendor := fs.String("vendor", "", "the vendor the package is for")
	bare := fs.Bool("bare", false, "a bare board's package: no BOM or placement list")
	px := fs.Int("px_per_mm", 20, "pixels per mm in the renders")
	out := fs.String("out", "", "the directory to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *zipPath == "" || *vendor == "" || *out == "" {
		return fmt.Errorf("--zip, --vendor and --out are needed")
	}
	data, err := os.ReadFile(*zipPath)
	if err != nil {
		return err
	}
	return report.Make(data, report.Options{
		Vendor:  *vendor,
		Bare:    *bare,
		PxPerMM: *px,
		Name:    filepath.Base(*zipPath),
	}, *out)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fab_report:", err)
		os.Exit(1)
	}
}
