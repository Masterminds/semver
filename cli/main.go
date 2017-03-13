//Package cmd implement a cli tool to sort, filter
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"

	"github.com/mh-cbon/semver/cli/cmd"
)

var version = "0.0.0"

func main() {
	doVersion := false

	cmd := cmd.CliCmd{}
	flag.BoolVar(&doVersion, "version", false, "Show version")
	flag.BoolVar(&cmd.Sort, "sort", false, "Sort input versions")
	flag.BoolVar(&cmd.Desc, "desc", false, "Sort versions descending")
	flag.BoolVar(&cmd.ShowInvalid, "invalid", false, "Show only invalid versions")
	flag.StringVar(&cmd.Constraint, "filter", "", "Filter versions matching given semver constraint")

	flag.Parse()

	dest := os.Stdout
	var src io.Reader
	if flag.NArg() > 0 {
		// expect input versions in the arguments.
		b := bytes.NewBuffer([]byte{})
		for _, v := range flag.Args() {
			b.Write([]byte(v + "\n"))
		}
		src = b
	} else {
		src = os.Stdin
	}

	if err := cmd.Exec(dest, src); err != nil {
		log.Printf("An error has occurred: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}
