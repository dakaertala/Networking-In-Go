package main

// TLV = Type Length Value
// This comment indicates that the code implements a
// Type-Length-Value encoding scheme,
// where data is serialized with a type identifier,
// length of the payload, and the payload itself.

// The serialization consists of the following parts
// Type: An identifier indicating the kind of
// data (e.g., BinaryType or StringType in the code).
// Length: The size of the data payload (in bytes).
// Value: The actual data payload.

// Why?
// TLV allows different types of data to be serialized
// into a single stream while preserving their identity
// and boundaries.

// TLV is ideal for evolving protocols or formats
// where future extensions are expected.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Constants defining the TLV types and constraints
const (
	// BinaryType is assigned the value 1 (iota + 1)
	// iota starts at 0, so BinaryType = 1, StringType = 2
	BinaryType uint8 = iota + 1
	StringType       // StringType is implicitly 2
	// MaxPayloadSize defines the maximum allowed payload
	// size (10 MB)
	// Keep this low to avoid memory exhaustion attack
	MaxPayloadSize uint32 = 10 << 20 // 10 MB
)

// Custom error for when the payload size exceeds MaxPayloadSize
var ErrMaxPayloadSize = errors.New("maximum payload size exceeded ")

// Payload interface defines the behavior for types
// that can be encoded/decoded in TLV format
type Payload interface {
	fmt.Stringer   // Requires String() method to return a string representation
	io.ReaderFrom  // Requires ReadFrom() to read from an io.Reader
	io.WriterTo    // Requires WriteTo() to write to an io.Writer
	Bytes() []byte // Returns the raw byte slice of the payload
}

// Binary type is a simple alias for a byte slice
type Binary []byte

// Bytes method returns the Binary payload as a byte slice
// Satisfies the Payload interface's Bytes() requirement
func (m Binary) Bytes() []byte {
	// Simply returns the underlying byte slice
	return m
}

// String method converts the Binary payload to a string
// Satisfies the fmt.Stringer interface
func (m Binary) String() string {
	// Converts the byte slice to a string
	return string(m)
}

// WriteTo serializes the Binary payload to an
// io.Writer in TLV format
// Satisfies the io.WriterTo interface
func (m Binary) WriteTo(w io.Writer) (int64, error) {
	// Write the type identifier (BinaryType = 1)
	// in big-endian format
	err := binary.Write(w, binary.BigEndian, BinaryType)
	if err != nil {
		// Return 0 bytes written and the error
		return 0, err
	}
	// Track the number of bytes written (1 byte for the type)
	var n int64 = 1

	// Write the length of the payload as a
	// uint32 in big-endian format
	err = binary.Write(w, binary.BigEndian, uint32(len(m)))
	if err != nil {
		// Return bytes written so far (1) and the error
		return n, err
	}
	// Add the 4 bytes written for the length field
	n += 4

	// Write the actual payload data
	output, err := w.Write(m)

	// Return total bytes written (type + length + payload)
	// and any error
	return n + int64(output), err
}

// ReadFrom deserializes a Binary payload from an
// io.Reader in TLV format
// Satisfies the io.ReaderFrom interface
// Note: m is a pointer receiver (*Binary)
// to modify the Binary value
func (m *Binary) ReadFrom(r io.Reader) (int64, error) {
	// Read the type identifier (1 byte)
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		// Return 0 bytes read and the error
		return 0, err
	}

	// Track the number of bytes read (1 byte for the type)
	var n int64 = 1
	// Verify the type is BinaryType (1)
	if typ != BinaryType {
		// Return bytes read and error if type mismatch
		return n, errors.New("invalid Binary")
	}

	// Read the length field (4 bytes) as a uint32
	var size uint32
	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		// Return bytes read so far (1) and the error
		return n, err
	}
	// Add the 4 bytes read for the length field
	n += 4

	// Check if the payload size exceeds the maximum allowed
	if size > MaxPayloadSize {
		// Return bytes read and max payload error
		return n, ErrMaxPayloadSize
	}

	// Allocate a byte slice of the specified size to
	// store the payload
	*m = make([]byte, size)
	// Read the payload data into the allocated slice
	output, err := r.Read(*m)

	// Return total bytes read (type + length + payload)
	// and any error
	return n + int64(output), err
}
