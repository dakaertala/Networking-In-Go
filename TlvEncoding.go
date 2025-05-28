package main

// TLV = Type Length Value

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	BinaryType uint8 = iota + 1
	StringType
	MaxPayloadSize uint32 = 10 << 20 // 10 MB
)

var ErrMaxPayloadSize = errors.New("maximum payload size exceeded ")

type Payload interface {
	fmt.Stringer
	io.ReaderFrom
	io.WriterTo
	Bytes() []byte
}

type Binary []byte

func (m Binary) Bytes() []byte { return m }

func (m Binary) String() string { return string(m) }

func (m Binary) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, BinaryType)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	err = binary.Write(w, binary.BigEndian, uint32(len(m)))
	if err != nil {
		return n, err
	}
	n += 4

	output, err := w.Write(m)

	return n + int64(output), err
}

func (m *Binary) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}

	var n int64 = 1
	if typ != BinaryType {
		return n, errors.New("invalid Binary")
	}

	var size uint32
	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return n, err
	}
	n += 4

	if size > MaxPayloadSize {
		return n, ErrMaxPayloadSize
	}

	*m = make([]byte, size)
	output, err := r.Read(*m)

	return n + int64(output), err
}
