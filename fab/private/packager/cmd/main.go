// SPDX-License-Identifier: Apache-2.0

// Command packager writes a fab house's ZIP for a board (issue #9):
//
//	packager --vendor jlcpcb --board NAME --bom BOM.csv --pos POS.csv \
//	         --fab DIR_OR_FILE... [--rotation FOOTPRINT_REGEXP=DEGREES]... \
//	         --out OUT.zip
//
// --fab names the Gerber and drill files, or directories of them (the
// outputs of kicad_gerbers and kicad_drill), walked in name order.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/filmil/bazel_rules_fab/fab/private/packager"
)

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

// readFab reads the files at paths, and the files under each directory.
func readFab(paths []string) ([]packager.File, error) {
	var out []packager.File
	for _, p := range paths {
		var names []string
		err := filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				names = append(names, q)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(names)
		for _, n := range names {
			b, err := os.ReadFile(n)
			if err != nil {
				return nil, err
			}
			out = append(out, packager.File{Name: filepath.ToSlash(n), Data: b})
		}
	}
	return out, nil
}

func run() error {
	vendor := flag.String("vendor", "", "the vendor: "+strings.Join(vendors(), ", "))
	board := flag.String("board", "", "the board's name, for the CSV files' names")
	bomPath := flag.String("bom", "", "kicad_bom's CSV")
	posPath := flag.String("pos", "", "kicad_pos's CSV")
	out := flag.String("out", "", "the ZIP to write")
	var fab, rots list
	flag.Var(&fab, "fab", "a Gerber or drill file, or a directory of them; repeat it")
	flag.Var(&rots, "rotation", "FOOTPRINT_REGEXP=DEGREES, a rotation correction; repeat it")
	flag.Parse()
	l, ok := packager.Layouts[*vendor]
	if !ok {
		return fmt.Errorf("--vendor %q: want one of %s", *vendor, strings.Join(vendors(), ", "))
	}
	if *board == "" || *bomPath == "" || *posPath == "" || *out == "" || len(fab) == 0 {
		return fmt.Errorf("--board, --bom, --pos, --fab and --out are all needed")
	}
	var rotations []packager.Rotation
	for _, r := range rots {
		rot, err := packager.ParseRotation(r)
		if err != nil {
			return err
		}
		rotations = append(rotations, rot)
	}
	bomIn, err := os.ReadFile(*bomPath)
	if err != nil {
		return err
	}
	posIn, err := os.ReadFile(*posPath)
	if err != nil {
		return err
	}
	bom, err := packager.BOM(l, bomIn)
	if err != nil {
		return err
	}
	cpl, err := packager.CPL(l, posIn, rotations)
	if err != nil {
		return err
	}
	files, err := readFab(fab)
	if err != nil {
		return err
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := packager.Zip(f, l, *board, files, bom, cpl); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func vendors() []string {
	var v []string
	for k := range packager.Layouts {
		v = append(v, k)
	}
	sort.Strings(v)
	return v
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "packager:", err)
		os.Exit(1)
	}
}
