//Package cmd implements a cli tool to sort, filter
package cmd

import (
	"bufio"
	"io"
	"sort"
	"strings"

	"github.com/mh-cbon/semver"
)

// CliCmd declares args for the cli
type CliCmd struct {
	Sort        bool
	ShowInvalid bool
	Desc        bool
	Constraint  string
}

// Exec runs the cmd, it returns an error only if the input arguments are incompatible.
func (c CliCmd) Exec(dest io.Writer, src io.Reader) error {
	src = newLineReader(src)
	src = newVersionValidatorReader(src, !c.ShowInvalid)
	if c.Sort {
		src = newVersionSorterReader(src, c.Desc)
	}
	dest = newLineWriter(dest)
	if c.Constraint != "" {
		c, err := semver.NewConstraint(c.Constraint)
		if err != nil {
			return err
		}
		dest = newVersionFilterWriter(dest, c)
	}

	_, err := io.Copy(dest, src)
	if err == io.EOF {
		err = nil
	}
	return err
}

// lineReader reads a reader by line
type lineReader struct {
	r    *bufio.Reader
	line []byte
}

// newLineReader makes a new LineReader of an io.Reader
func newLineReader(r io.Reader) *lineReader {
	return &lineReader{r: bufio.NewReader(r)}
}

// ReadLine returns the next line, if line is empty, you should skip the iteration.
//you should check errors on every line.
func (l *lineReader) Read(p []byte) (int, error) {

	line, isPrefix, err := l.r.ReadLine()

	n := 0

	if isPrefix {
		l.line = append(l.line, line...)
	} else if len(l.line) > 0 {
		n = len(l.line)
		copy(p, l.line)
		l.line = l.line[:0]
	} else {
		n = len(line)
		copy(p, line)
	}

	if err == io.EOF {
		// fmt.Println(string(p))
		// fmt.Println(string(line))
		// fmt.Println(string(l.line))
		n = len(l.line)
		copy(p, l.line)
	}

	return n, err
}

// versionValidatorReader reads a reader by line
type versionValidatorReader struct {
	r         io.Reader
	validOnly bool
}

// newVersionValidatorReader reads chunks as version
func newVersionValidatorReader(r io.Reader, valid bool) *versionValidatorReader {
	return &versionValidatorReader{r: r, validOnly: valid}
}

// Read p as a version, emits p only if it matches validOnly
func (l *versionValidatorReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if n > 0 {
		_, err2 := semver.NewVersion(strings.TrimSpace(string(p[:n])))
		keepRead := (l.validOnly && err2 == nil) || (!l.validOnly && err2 != nil)
		if keepRead == false {
			p = p[0:0]
			n = len(p)
		}
	}
	return n, err
}

// versionSorterReader reads versions and emits them sorted.
// by design it will buffer!
type versionSorterReader struct {
	r         io.Reader
	desc      bool
	collected []string
	doEmit    bool
}

// newVersionSorterReader makes a new versionSorterReader of an io.Reader
func newVersionSorterReader(r io.Reader, desc bool) *versionSorterReader {
	return &versionSorterReader{r: r, desc: desc, collected: []string{}}
}

// Read buffers input p, assuming they are valid versions, on EOF, it starts emits version.
func (l *versionSorterReader) Read(p []byte) (int, error) {
	var n int
	var err error

	if l.doEmit == false {
		n, err = l.r.Read(p)
		if n > 0 {
			l.collected = append(l.collected, string(p[0:n]))
			p = p[0:0]
			n = len(p)
		}
		if err == io.EOF {
			// flush time!
			err = nil
			l.doEmit = true
			vs := make([]*semver.Version, len(l.collected))
			for i, r := range l.collected {
				v, _ := semver.NewVersion(r)
				vs[i] = v
			}
			if l.desc {
				sort.Sort(sort.Reverse(semver.Collection(vs)))
			} else {
				sort.Sort(semver.Collection(vs))
			}
			for i, v := range vs {
				l.collected[i] = v.String()
			}
		}
	}

	if l.doEmit {
		if len(l.collected) > 0 {
			s := []byte(l.collected[0])
			l.collected = l.collected[1:]
			// if len(l.collected) > 0 {
			// }
			p = p[0:len(s)]
			copy(p, s)
			n = len(p)
		} else {
			p = p[0:0]
			n = len(p)
			err = io.EOF
		}
	}
	return n, err
}

// lineWriter reads a reader by line
type lineWriter struct {
	w io.Writer
}

// newLineReader makes a new LineReader of an io.Reader
func newLineWriter(w io.Writer) *lineWriter {
	return &lineWriter{w: w}
}

// ReadLine returns the next line, if line is empty, you should skip the iteration.
//you should check errors on every line.
func (l *lineWriter) Write(p []byte) (int, error) {
	oLen := len(p)
	n, err := l.w.Write(p)
	if err == nil && oLen == n {
		// that would need more test.
		_, err = l.w.Write([]byte("\n"))
	}
	return n, err
}

type versionFilterWriter struct {
	dest       io.Writer
	constraint *semver.Constraints
}

func newVersionFilterWriter(dest io.Writer, constraint *semver.Constraints) *versionFilterWriter {
	return &versionFilterWriter{dest: dest, constraint: constraint}
}
func (w *versionFilterWriter) Write(p []byte) (int, error) {
	// it is expected that p is a version string
	v, _ := semver.NewVersion(string(p))
	doWrite := w.constraint.Check(v)
	//
	// fmt.Printf("%q", string(p))
	// fmt.Printf("%v", doWrite)
	// fmt.Printf("%#v", w.constraint)

	var n int
	var err error
	if doWrite {
		n, err = w.dest.Write(p)
		// fmt.Println(n)
		// fmt.Println(err)
	} else {
		// fmt.Println("=====> skipped")
		// make it believe write occurred to avoid ErrShortWrite
		n = len(p)
		err = nil
	}

	return n, err
}

//
// type versionValidatorWriter struct {
// 	dest      io.Writer
// 	validOnly bool
// }
//
// func newVersionValidatorWriter(dest io.Writer, valid bool) *versionValidatorWriter {
// 	return &versionValidatorWriter{dest: dest, validOnly: valid}
// }
// func (w *versionValidatorWriter) Write(p []byte) (int, error) {
// 	// it is expected that p is a version string
// 	_, err := semver.NewVersion(string(p))
// 	doWrite := (w.validOnly && err == nil) || (!w.validOnly && err != nil)
//
// 	var n int
// 	if doWrite {
// 		n, err = w.dest.Write(p)
// 		// fmt.Println(n)
// 		// fmt.Println(err)
// 	} else {
// 		fmt.Println("=====> skipped")
// 		// make it believe write occurred to avoid ErrShortWrite
// 		n = len(p)
// 		err = nil
// 	}
//
// 	return n, err
// }
