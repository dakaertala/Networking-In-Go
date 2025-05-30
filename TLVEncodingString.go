package main

import (
	"encoding/binary"
	"io"
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


