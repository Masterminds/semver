package stream

import (
	"bytes"
	"fmt"
	"io"
)

// Flusher can Flush.
type Flusher interface {
	Flush() error
}

// Sinker can Sink to a Flusher.
type Sinker interface {
	Sink(Flusher)
}

// Piper is a Flusher+Sinker that can Pipe/Unpipe.
type Piper interface {
	Flusher
	Sinker
	Pipe(Piper) Piper
	Unpipe(Flusher)
}

// ByteWriter receives bytes chunks
type ByteWriter interface {
	Flusher
	Write(d []byte) error
}

// ByteStream receives bytes chunks, writes it to the connected Pipes.
type ByteStream struct {
	Streams []ByteWriter
}

// Pipe connects a Pipe, returns the connected Pipe left-end.
func (p *ByteStream) Pipe(s Piper) Piper {
	// add lock
	p.Sink(s)
	return s
}

// Sink connects an ending Piper.
func (p *ByteStream) Sink(s Flusher) {
	x, ok := s.(ByteWriter)
	if !ok {
		panic("nop")
	}
	p.Streams = append(p.Streams, x)
}

// Unpipe disconnect a connected Pipe.
func (p *ByteStream) Unpipe(s Flusher) {
	// add lock
	x, ok := s.(ByteWriter)
	if !ok {
		panic("nop")
	}
	i := -1
	for e, pp := range p.Streams {
		if pp == x {
			i = e
			break
		}
	}
	if i > -1 {
		p.Streams = append(p.Streams[:i], p.Streams[i+1:]...)
	}
	fmt.Println(i)
}

// Flush flushes the connected Pipes.
func (p *ByteStream) Flush() error {
	for _, pp := range p.Streams {
		if err := pp.Flush(); err != nil {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

// Write a bytes chunk on the connected Pipes.
func (p *ByteStream) Write(d []byte) error {
	for _, pp := range p.Streams {
		if err := pp.Write(d); err != nil {
			return err
		}
	}
	return nil
}

// ByteReader consumes an io.Reader, writes bytes chunks to the connected Pipes.
type ByteReader struct {
	ByteStream
	closed bool
	r      io.Reader
}

// NewByteReader constructs a ByteReader of given io.Reader.
func NewByteReader(r io.Reader) *ByteReader {
	return &ByteReader{r: r}
}

// Consume the io.Reader until EOF, or Closed stream. It always flushes the Pipe.
func (p *ByteReader) Consume() error {
	var err error
	var n int
	data := make([]byte, 1024)
	for {
		n, err = p.r.Read(data)
		p.closed = err == io.EOF || n == 0
		data = data[0:n]
		// <-time.After(1 * time.Second) // blah.
		if err2 := p.Write(data); err2 != nil {
			p.closed = err2 == io.EOF
			err = err2
		}
		if p.closed {
			err = p.Flush()
			if x, ok := p.r.(io.Closer); ok {
				if err2 := x.Close(); err2 != nil {
					err = err2
				}
			}
			break
		}
	}

	if err == io.EOF {
		err = nil
	}

	return err
}

// type ByteReaderCloser struct {
// 	ByteStream
// 	closed bool
// 	a      *AsyncReader
// }
//
// func NewByteReaderCloser(r io.Reader) *ByteReaderCloser {
// 	return &ByteReaderCloser{a: NewAsyncReader(r)}
// }
//
// func (p *ByteReaderCloser) Consume() error {
// 	var err error
// 	finished := false
//
// 	for {
// 		if finished {
// 			break
// 		}
// 		select {
// 		case op := <-p.a.Read:
// 			data := op.Data
// 			// n := op.N
// 			err = *op.Err
// 			p.closed = err == io.EOF
// 			if err2 := p.Write(data); err2 != nil {
// 				err = err2
// 				p.closed = err == io.EOF
// 			}
// 		default:
// 			if p.closed {
// 				err = p.Flush()
// 				finished = true
// 				if err3 := p.a.Close(); err3 != nil {
// 					return err3
// 				}
// 			} else {
// 				// <-time.After(1 * time.Microsecond) // blah.
// 			}
// 		}
// 	}
//
// 	return err
// }
// func (p *ByteReaderCloser) Close() error {
// 	if p.closed {
// 		return ErrAlreadyClosed
// 	}
// 	p.closed = true
// 	return nil
// }
// func (p *ByteReaderCloser) CloseOn(f func()) Piper {
// 	go func() {
// 		f()
// 		p.Close()
// 	}()
// 	return p
// }

// ByteSink consumes an io.Writer.
type ByteSink struct {
	w     io.Writer
	onErr func(Flusher, error) error
}

// NewByteSink constructs a new ByteSink to consume an io.Writer.
func NewByteSink(w io.Writer) *ByteSink {
	return &ByteSink{w: w}
}

// OnError registers the callback to catch write errors.
func (p *ByteSink) OnError(f func(Flusher, error) error) *ByteSink {
	p.onErr = f
	return p
}

// Write bytes chunk on the underlying io.Writer.
func (p *ByteSink) Write(d []byte) error {
	_, err := p.w.Write(d)
	if err != nil && p.onErr != nil {
		err = p.onErr(p, err)
	}
	return err
}

// Flush is a no-op.
func (p *ByteSink) Flush() error {
	return nil
}

// BytesSplitter receives bytes chunks, split them given s...byte, writes each new chunk to the connected Pipes.
type BytesSplitter struct {
	ByteStream
	buf []byte
	s   []byte
}

// NewBytesSplitter constructs a BytesSplitter by any byte in s.
func NewBytesSplitter(s ...byte) *BytesSplitter {
	return &BytesSplitter{
		s: s,
	}
}

// Write a chunk bytes, split it by s, writes every new chunks to the connected Pipes.
func (p *BytesSplitter) Write(d []byte) error {
	p.buf = append(p.buf, d...)
	q := false
	for {
		if q {
			break
		}
		for _, s := range p.s {
			if i := bytes.IndexByte(p.buf, s); i >= 0 {
				data := p.buf[:i]
				if len(data) > 0 {
					if err := p.ByteStream.Write(data); err != nil {
						return err
					}
				}
				p.buf = p.buf[i+1:]
			} else {
				q = true
			}
		}
	}
	return nil
}

// Flush the underlying buffer to the connected Pipes.
func (p *BytesSplitter) Flush() error {
	q := false
	for {
		if q {
			break
		}
		for _, s := range p.s {
			if i := bytes.IndexByte(p.buf, s); i >= 0 {
				data := p.buf[:i]
				if len(data) > 0 {
					if err := p.ByteStream.Write(data); err != nil {
						return err
					}
				}
				p.buf = p.buf[i+1:]
			} else {
				q = true
			}
		}
	}

	if len(p.buf) > 0 {
		if err := p.ByteStream.Write(p.buf); err != nil {
			return err
		}
	}
	return p.ByteStream.Flush()
}

// SplitBytesByLine receives bytes chunks, split them by line where EOL is (\n\r?), writes each line as a bytes chunk to the connected Pipes.
type SplitBytesByLine struct {
	ByteStream
	buf []byte
}

// Write buffers the bytes chunk, split it by line, writes each line as a bytes chunk to the connected Pipes.
func (p *SplitBytesByLine) Write(d []byte) error {
	p.buf = append(p.buf, d...)
	for {
		if i := bytes.IndexByte(p.buf, '\n'); i >= 0 {
			data := dropCR(p.buf[:i])
			if len(data) > 0 {
				if err := p.ByteStream.Write(data); err != nil {
					return err
				}
			}
			p.buf = p.buf[i+1:]
		} else {
			break
		}
	}
	return nil
}

// Flush writes the underlying buffer to the connected Pipes.
func (p *SplitBytesByLine) Flush() error {
	if len(p.buf) > 0 {
		if err := p.ByteStream.Write(dropCR(p.buf[0:])); err != nil {
			return err
		}
	}
	return p.ByteStream.Flush()
}

// BytesTrimer receives bytes chunks, trim their whitespaces, writes every chunks to the connected Pipes.
type BytesTrimer struct {
	ByteStream
}

// NewBytesTrimer constructs a BytesTrimer to prefix and suffix the chunks.
func NewBytesTrimer() *BytesTrimer {
	return &BytesTrimer{}
}

// Write trims given chunk, writes the chunk on the connected Pipes.
func (p *BytesTrimer) Write(d []byte) error {
	return p.ByteStream.Write(bytes.TrimSpace(d))
}

// BytesPrefixer receives bytes chunks, prefix and suffix them, writes every chunks to the connected Pipes.
type BytesPrefixer struct {
	ByteStream
	prefix []byte
	suffix []byte
	b      *bytes.Buffer
}

// NewBytesPrefixer constructs a BytesPrefixer to prefix and suffix the chunks.
func NewBytesPrefixer(prefix string, suffix string) *BytesPrefixer {
	return &BytesPrefixer{
		prefix: []byte(prefix),
		suffix: []byte(suffix),
		b:      bytes.NewBuffer(make([]byte, 1024)),
	}
}

// Write prefixes and suffixes given chunk, writes the chunk on the connected Pipes.
func (p *BytesPrefixer) Write(d []byte) error {
	p.b.Truncate(0)
	p.b.Write(p.prefix)
	p.b.Write(d)
	p.b.Write(p.suffix)
	return p.ByteStream.Write(p.b.Bytes())
}

// FirstChunkOnly receives bytes chunks, writes only the first chunk on the connected Pipes.
type FirstChunkOnly struct {
	ByteStream
	d bool
}

// Write only the first bytes chunk to the connected Pipes.
func (p *FirstChunkOnly) Write(d []byte) error {
	if !p.d {
		p.d = true
		return p.ByteStream.Write(d)
	}
	return nil
}

// LastChunkOnly receives bytes chunks, writes only the last chunk to the conencted Pipes.
type LastChunkOnly struct {
	ByteStream
	d     bool
	chunk []byte
}

// Write buffer given bytes chunk.
func (p *LastChunkOnly) Write(d []byte) error {
	p.d = true
	p.chunk = append(p.chunk[:0], d...) // need to copy ?
	return nil
}

// Flush writes the last chunk on the conncted Pipes.
func (p *LastChunkOnly) Flush() error {
	if p.d {
		if err := p.ByteStream.Write(p.chunk); err != nil {
			return err
		}
	}
	return p.ByteStream.Flush()
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}
