package stream

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver"
)

// VersionPipeWriter receives *Version
type VersionPipeWriter interface {
	Flusher
	Write(*semver.Version) error
}

// VersionStream receives *Version, writes it to the connected Pipes.
type VersionStream struct {
	Streams []VersionPipeWriter
}

// Pipe connects a Pipe, returns the connected Pipe left-end.
func (p *VersionStream) Pipe(s Piper) Piper {
	p.Sink(s)
	return s
}

// Sink connects an ending Piper.
func (p *VersionStream) Sink(s Flusher) {
	// add lock
	x, ok := s.(VersionPipeWriter)
	if !ok {
		fmt.Printf("from %T\n", p)
		fmt.Printf("to %T\n", s)
		panic("nop")
	}
	p.Streams = append(p.Streams, x)
}

// Unpipe disconnect a connected Pipe.
func (p *VersionStream) Unpipe(s Flusher) {
	// add lock
}

// Flush flushes the connected Pipes.
func (p *VersionStream) Flush() error {
	for _, pp := range p.Streams {
		if err := pp.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// Write a *Version on the connected Pipes.
func (p *VersionStream) Write(d *semver.Version) error {
	for _, pp := range p.Streams {
		if err := pp.Write(d); err != nil {
			return err
		}
	}
	return nil
}

// VersionFromByte receives bytes encoded *Version, pushes *Version
type VersionFromByte struct {
	VersionStream
}

// Write receive a chunk of []byte, writes a *Version on the connected Pipes.
func (p *VersionFromByte) Write(d []byte) error {
	s, err := semver.NewVersion(string(d))
	if err != nil {
		return fmt.Errorf("Invalid version %q", string(d))
	}
	return p.VersionStream.Write(s)
}

// VersionConstraint receives *Version, when it satisfies a Constraint, writes the *Version on the connected Pipes.
type VersionConstraint struct {
	VersionStream
	c *semver.Constraints
}

// NewVersionContraint is a ctor.
func NewVersionContraint(c *semver.Constraints) *VersionConstraint {
	return &VersionConstraint{c: c}
}

// Write the *Version s on the connected Pipes, when it satisfies the Constraint
func (p *VersionConstraint) Write(v *semver.Version) error {
	if p.c.Check(v) {
		return p.VersionStream.Write(v)
	}
	return nil
}

// VersionSorter receives *Version, buffer them until flush, order all *Versions, writes all *Version to the connected Pipes.
type VersionSorter struct {
	VersionStream
	all []*semver.Version
	Asc bool
}

// Write *Version to the buffer.
func (p *VersionSorter) Write(v *semver.Version) error {
	p.all = append(p.all, v)
	return nil
}

// Flush sorts all buffered *Version, writes all *Version to the connected Pipes.
func (p *VersionSorter) Flush() error {
	if p.Asc {
		sort.Sort(semver.Collection(p.all))
	} else {
		sort.Sort(sort.Reverse(semver.Collection(p.all)))
	}
	for _, v := range p.all {
		p.VersionStream.Write(v)
	}
	p.all = p.all[:0]
	return p.VersionStream.Flush()
}

// InvalidVersionFromByte receives bytes chunks of *Version, when it fails to decode it as a *Version, writes the chunk on the connected Pipes.
type InvalidVersionFromByte struct {
	ByteStream
}

// Write a chunk of bytes, when it is not a valid *Version, writes the chunk on the connected Pipes.
func (p *InvalidVersionFromByte) Write(d []byte) error {
	_, err := semver.NewVersion(string(d))
	if err == nil {
		return nil
	}
	return p.ByteStream.Write(d)
}

// VersionToByte receives *Version, writes bytes chunks to the connection Pipes.
type VersionToByte struct {
	ByteStream
}

// Write encode *Version to a byte chunk, writes the chunk to the connected Pipes.
func (p *VersionToByte) Write(d *semver.Version) error {
	return p.ByteStream.Write([]byte(d.String()))
}
