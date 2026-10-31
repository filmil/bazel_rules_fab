// SPDX-License-Identifier: Apache-2.0

// Command pkgcheck is fab_package_test's checker. It reads, from the
// environment:
//
//   - PKG_ZIP: the ZIP, relative to the runfiles directory;
//   - PKG_VENDOR: the vendor;
//   - PKG_WANT_ERROR: empty for a good ZIP; else text that one of the
//     problems must contain (a known-bad ZIP);
//   - PKG_BARE: "1" for a bare board's ZIP, with no BOM or placement list.
package main

import (
	"fmt"
	"os"

	"github.com/filmil/bazel_rules_fab/fab/private/pkgcheck"
)

func run() error {
	data, err := os.ReadFile(os.Getenv("PKG_ZIP"))
	if err != nil {
		return err
	}
	problems, err := pkgcheck.Check(os.Getenv("PKG_VENDOR"), data, os.Getenv("PKG_BARE") == "1")
	if err != nil {
		return err
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if err := pkgcheck.Verdict(problems, os.Getenv("PKG_WANT_ERROR")); err != nil {
		return fmt.Errorf("%s for %s: %v", os.Getenv("PKG_ZIP"), os.Getenv("PKG_VENDOR"), err)
	}
	fmt.Println("as expected")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fab_package_test:", err)
		os.Exit(1)
	}
}
