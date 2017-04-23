//Package cmd implement a cli tool to manipulate Versions.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Masterminds/semver"
	"github.com/mh-cbon/semver/cmd/stream"
)

var version = "0.0.0"
var name = "semver"

type cliOpts struct {
	help        bool
	version     bool
	sort        bool
	s           bool
	valid       bool
	showInvalid bool
	desc        bool
	d           bool
	constraints string
	c           string
	first       bool
	f           bool
	last        bool
	l           bool
	json        bool
	j           bool
}

func main() {
	opts := cliOpts{}
	flag.BoolVar(&opts.version, "version", false, "Show version")
	flag.BoolVar(&opts.help, "help", false, "Show help")

	flag.BoolVar(&opts.valid, "valid", false, "Emit error on invalid version")
	flag.BoolVar(&opts.sort, "sort", false, "Sort input versions")
	flag.BoolVar(&opts.s, "s", false, "Alias -s")
	flag.BoolVar(&opts.desc, "desc", false, "Sort versions descending")
	flag.BoolVar(&opts.d, "d", false, "Alias -desc")

	flag.BoolVar(&opts.showInvalid, "invalid", false, "Show only invalid versions")

	flag.StringVar(&opts.constraints, "filter", "", "Filter versions matching given semver constraint")
	flag.StringVar(&opts.c, "c", "", "Alias -filter")

	flag.BoolVar(&opts.last, "last", false, "Only last version")
	flag.BoolVar(&opts.l, "l", false, "Alias -last")

	flag.BoolVar(&opts.first, "first", false, "Only first version")
	flag.BoolVar(&opts.f, "f", false, "Alias -first")

	flag.BoolVar(&opts.json, "json", false, "JSON output")
	flag.BoolVar(&opts.j, "j", false, "Alias -json")

	flag.Parse()

	if opts.version {
		showVer()
		os.Exit(0)
	}
	if opts.help {
		showHelp()
		os.Exit(0)
	}

	var src io.Reader
	dest := os.Stdout

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

	pipeSrc := stream.NewByteReader(src)
	pipe := pipeSrc.
		Pipe(stream.NewBytesSplitter(' ', '\n')).
		Pipe(&stream.BytesTrimer{})

	if opts.showInvalid {
		pipe = pipe.Pipe(&stream.InvalidVersionFromByte{})

		if opts.first || opts.f {
			pipe = pipe.Pipe(&stream.FirstChunkOnly{})
		} else if opts.last || opts.l {
			pipe = pipe.Pipe(&stream.LastChunkOnly{})
		}

	} else {
		pipe = pipe.Pipe(&stream.VersionFromByte{SkipInvalid: !opts.valid})

		c := getConstraint(opts)
		if c != nil {
			pipe = pipe.Pipe(stream.NewVersionContraint(c))
		}

		if opts.sort || opts.s {
			pipe = pipe.Pipe(&stream.VersionSorter{Asc: !(opts.desc || opts.d)})
		}

		if opts.first || opts.f {
			pipe = pipe.Pipe(&stream.FirstVersionOnly{})
		} else if opts.last || opts.l {
			pipe = pipe.Pipe(&stream.LastVersionOnly{})
		}

		if opts.json || opts.j {
			pipe = pipe.Pipe(&stream.VersionJsoner{})
		} else {
			pipe = pipe.Pipe(&stream.VersionToByte{})
		}
	}

	if !opts.json || opts.j {
		pipe = pipe.Pipe(stream.NewBytesPrefixer("- ", "\n"))
	}

	pipe.Sink(stream.NewByteSink(dest))

	if err := pipeSrc.Consume(); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func getConstraint(opts cliOpts) *semver.Constraints {
	var c *semver.Constraints
	var err error
	if opts.constraints != "" {
		c, err = semver.NewConstraint(opts.constraints)
	} else if opts.c != "" {
		c, err = semver.NewConstraint(opts.c)
	}

	if err != nil {
		panic(err)
	}
	return c
}

func showVer() {
	fmt.Printf("%v - %v\n", name, version)
}
func showHelp() {
	showVer()
	fmt.Printf(`
Usage

	-filter|-c  string  Filter versions matching given semver constraint
	-invalid    bool    Show only invalid versions

	-sort|-s    bool    Sort input versions
	-desc|-d    bool    Sort versions descending

	-first|-f   bool    Only first version
	-last|-l    bool    Only last version

	-json|-j    bool    JSON output

	-version    bool    Show version

Example

	semver -c 1.x 0.0.4 1.2.3
	echo "0.0.4 1.2.3" | semver -j
	echo "0.0.4 1.2.3" | semver -s
	echo "0.0.4 1.2.3" | semver -s -d -j -f
	echo "0.0.4 1.2.3 tomate" | semver -invalid
`)
}
