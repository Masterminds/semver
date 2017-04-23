//Package cmd implement a cli tool to sort, filter
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mh-cbon/semver/cmd/stream"
)

var version = "0.0.0"

type cliOpts struct {
	version     bool
	sort        bool
	showInvalid bool
	desc        bool
	constraint  string
	first       bool
	last        bool
	json        bool
}

func main() {
	opts := cliOpts{}
	flag.BoolVar(&opts.version, "version", false, "Show version")
	flag.BoolVar(&opts.sort, "sort", false, "Sort input versions")
	flag.BoolVar(&opts.desc, "desc", false, "Sort versions descending")
	flag.BoolVar(&opts.showInvalid, "invalid", false, "Show only invalid versions")
	flag.StringVar(&opts.constraint, "filter", "", "Filter versions matching given semver constraint")
	flag.BoolVar(&opts.json, "json", false, "JSON output")

	flag.Parse()

	fmt.Printf("%#v\n", opts)

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
		Pipe(&stream.VersionFromByte{})

	if opts.sort {
		pipe = pipe.Pipe(&stream.VersionSorter{Asc: !opts.desc}).Pipe(&stream.VersionToByte{})
	} else if opts.showInvalid {
		pipe = pipe.Pipe(&stream.InvalidVersionFromByte{})
	}

	if opts.first {
		pipe = pipe.Pipe(&stream.FirstChunkOnly{})
	} else if opts.last {
		pipe = pipe.Pipe(&stream.LastChunkOnly{})
	}

	if opts.json {
		// tbd.
	} else {
		pipe = pipe.Pipe(stream.NewBytesPrefixer("- ", "\n"))
	}

	pipe.Sink(stream.NewByteSink(dest))

	if err := pipeSrc.Consume(); err != nil {
		panic(err)
	}

}
