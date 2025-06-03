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

type String string

func (m String) Bytes() []byte {
	return []byte(m)
}

func (m String) String() string {
	return string(m)
}

func (m String) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, StringType) // 1-byte
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	err = binary.Write(w, binary.BigEndian, uint32(len(m))) // 4-bytes
	if err != nil {
		return n, err
	}

	n += 4

	output, err := w.Write([]byte(m))

	return n + int64(output), err
}

func (m *String) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	if typ != StringType {
		return n, errors.New("invalid String")
	}

	var size uint32
	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return n, err
	}
	n += 4

	buf := make([]byte, size)
	output, err := r.Read(buf)
	if err != nil {
		return n, err
	}
	*m = String(buf)

	return n + int64(output), nil
}

func decode(r io.Reader) (Payload, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return nil, err
	}

	var payload Payload

	switch typ {
	case BinaryType:
		payload = new(Binary)
	case StringType:
		payload = new(String)
	default:
		return nil, errors.New("unkown type")
	}

	_, err = payload.ReadFrom(io.MultiReader(bytes.NewReader([]byte{typ}), r))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func TestPayloads(t *testing.T) {
	b1 := Binary("Clear is better than clever.")
	b2 := binary("Don't panic.")
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
