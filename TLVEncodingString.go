package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
)

// Define a new type String as an alias for the built-in string type
type String string

// Bytes returns the byte slice representation of the String
func (m String) Bytes() []byte {
	return []byte(m)
}

// String returns the underlying string value of the String type
func (m String) String() string {
	return string(m)
}

// WriteTo writes the encoded String to an io.Writer.
// It encodes a type marker, the length of the string, and the string bytes themselves.
// Returns the number of bytes written and an error if any.
func (m String) WriteTo(w io.Writer) (int64, error) {
	// Write the type marker byte identifying this payload as a String type
	err := binary.Write(w, binary.BigEndian, StringType) // 1-byte
	if err != nil {
		return 0, err
	}
	// Count bytes written so far (1 byte for type)
	var n int64 = 1

	// Write the length of the string as a 4-byte unsigned
	// integer (BigEndian)
	err = binary.Write(w, binary.BigEndian, uint32(len(m))) // 4-bytes
	if err != nil {
		return n, err
	}
	// Add 4 bytes to the count for the length field
	n += 4

	// Write the actual string bytes
	output, err := w.Write([]byte(m))
	// output is the number of bytes written for the string contents

	// Return total bytes written and error if any
	return n + int64(output), err
}

// ReadFrom reads an encoded String from an io.Reader.
// It expects the input format to be:
// [1-byte type marker][4-byte length][string bytes]
// It validates the type marker and reads the string bytes accordingly.
// Returns the number of bytes read and an error if any.
func (m *String) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	// Read 1 byte to get the type marker
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		// Return error if reading fails
		return 0, err
	}
	// Count bytes read so far (1 byte for type)
	var n int64 = 1

	// Validate the type marker to ensure it matches StringType
	if typ != StringType {
		return n, errors.New("invalid String")
	}

	var size uint32
	// Read 4 bytes for the length of the string
	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return n, err
	}
	// Add 4 bytes read for length
	n += 4

	// Allocate a buffer to hold the string bytes
	// based on the length
	buf := make([]byte, size)
	// Read the string bytes into the buffer
	output, err := r.Read(buf)
	if err != nil {
		return n, err
	}

	// Assign the read bytes converted to String type
	// back to receiver
	*m = String(buf)

	// Return total bytes read and nil error
	return n + int64(output), nil
}

// decode reads a type marker byte from the reader,
// creates an instance of the appropriate Payload type,
// and delegates the reading of the full payload to that type.
// Returns the decoded Payload or an error.
func decode(r io.Reader) (Payload, error) {
	var typ uint8
	// Read 1 byte for the type marker to identify payload type
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return nil, err
	}

	var payload Payload

	// Instantiate the correct payload struct based on type marker
	switch typ {
	case BinaryType:
		// Create a new Binary instance
		payload = new(Binary)
	case StringType:
		// Create a new String instance
		payload = new(String)
	default:
		return nil, errors.New("unkown type")
	}

	// Use io.MultiReader to prepend the type byte back to the reader,
	// since ReadFrom expects the type byte to be read first
	_, err = payload.ReadFrom(io.MultiReader(bytes.NewReader([]byte{typ}), r))
	if err != nil {
		return nil, err
	}

	// Return the fully decoded payload
	return payload, nil
}

// Test of the TLC encoding
func TestPayloads(t *testing.T) {
	b1 := Binary("Clear is better than clever.")
	b2 := Binary("Don't panic.")
	s1 := String("Errors are values.")
	payloads := []Payload{&b1, &s1, &b2}

	listener, err := net.Listen("tcp", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()

		for _, p := range payloads {
			_, err := p.WriteTo(conn)
			if err != nil {
				t.Error(err)
				break
			}
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for i := 0; i < len(payloads); i++ {
		actual, err := decode(conn)
		if err != nil {
			t.Fatal(err)
		}

		if expected := payloads[i]; !reflect.DeepEqual(expected, actual) {
			t.Errorf("value mismatch: %v = %v", expected, actual)
			continue
		}

		t.Logf("[%T] %[1]q", actual)
	}
}
