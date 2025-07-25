package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

// DatagramSize is the maximum size of a TFTP packet.
// TFTP uses 512 bytes of data + 4 bytes of header (opcode + block number).
const (
	DatagramSize = 516              // 2 bytes opcode + 2 bytes block number + 512 bytes of data
	BlockSize    = DatagramSize - 4 // max data payload in a DATA packet (512 bytes)
)

// OpCode defines the possible TFTP operation codes (first 2 bytes in any TFTP packet).
type OpCode uint16

const (
	OpRRQ  OpCode = iota + 1 // Read Request (RRQ) — opcode = 1
	_                        // Write Request (WRQ) is intentionally skipped (not implemented)
	OpData                   // Data packet — opcode = 3
	OpAck                    // Acknowledgment — opcode = 4
	OpErr                    // Error packet — opcode = 5
)

// ErrCode defines standard TFTP error codes used in ERROR packets.
type ErrCode uint16

const (
	ErrUnknown         ErrCode = iota // 0: Not defined, see error message
	ErrNotFound                       // 1: File not found
	ErrAccessViolation                // 2: Access violation
	ErrDiskFull                       // 3: Disk full or allocation exceeded
	ErrIllegalOp                      // 4: Illegal TFTP operation
	ErrUnknownID                      // 5: Unknown transfer ID
	ErrFileExists                     // 6: File already exists
	ErrNoUser                         // 7: No such user
)

// ReadReq represents a TFTP Read Request (RRQ).
// It includes a filename and a transfer mode (usually "octet" for binary).
type ReadReq struct {
	Filename string
	Mode     string
}

// MarshalBinary serializes the ReadReq into a binary format that conforms to the TFTP RRQ specification.
// The layout is: [2 bytes opcode][filename][0][mode][0]
func (q ReadReq) MarshalBinary() ([]byte, error) {
	// Default to "octet" mode if not specified
	mode := "octet"
	if q.Mode != "" {
		mode = q.Mode
	}

	// Estimate buffer capacity:
	//   2 bytes opcode + len(filename) + 1 (null byte) + len(mode) + 1 (null byte)
	cap := 2 + len(q.Filename) + 1 + len(mode) + 1
	b := new(bytes.Buffer)
	b.Grow(cap) // Avoid reallocations

	// Write the opcode (1 for RRQ) in big-endian byte order
	if err := binary.Write(b, binary.BigEndian, OpRRQ); err != nil {
		return nil, err
	}

	// Write the filename followed by a null terminator
	if _, err := b.WriteString(q.Filename); err != nil {
		return nil, err
	}
	if err := b.WriteByte(0); err != nil {
		return nil, err
	}

	// Write the mode string (e.g., "octet") followed by a null terminator
	if _, err := b.WriteString(mode); err != nil {
		return nil, err
	}
	if err := b.WriteByte(0); err != nil {
		return nil, err
	}

	// Return the constructed byte slice
	return b.Bytes(), nil
}

// UnmarshalBinary deserializes a byte slice into a ReadReq struct, validating the format.
// It expects a valid RRQ format: [2 bytes opcode][filename][0][mode][0]
func (q *ReadReq) UnmarshalBinary(p []byte) error {
	r := bytes.NewBuffer(p) // Wrap input bytes in a buffer for easier reading

	var code OpCode
	// Read the 2-byte opcode and check it's a Read Request (RRQ)
	if err := binary.Read(r, binary.BigEndian, &code); err != nil {
		return err
	}
	if code != OpRRQ {
		return errors.New("invalid RRQ")
	}

	// Read the filename (up to null byte), then trim the null terminator
	filename, err := r.ReadString(0)
	if err != nil {
		return errors.New("invalid RRQ")
	}
	q.Filename = strings.TrimRight(filename, "\x00")
	if len(q.Filename) == 0 {
		return errors.New("invalid RRQ: empty filename")
	}

	// Read the mode (e.g., "octet") up to the null byte
	mode, err := r.ReadString(0)
	if err != nil {
		return errors.New("invalid RRQ")
	}
	q.Mode = strings.TrimRight(mode, "\x00")

	// Only "octet" mode is supported for binary transfers
	actual := strings.ToLower(q.Mode)
	if actual != "octet" {
		return errors.New("only binary transfers supported")
	}

	return nil
}

type Data struct {
	Block   uint16    // Block number of this data packet (starts from 1)
	Payload io.Reader // Reader that supplies the data payload (up to 512 bytes)
}

// MarshalBinary converts the Data struct into a TFTP DATA packet binary format.
// The layout is: [2 bytes opcode][2 bytes block number][<=512 bytes payload]
func (d *Data) MarshalBinary() ([]byte, error) {
	// Create a buffer and preallocate capacity to avoid resizing
	b := new(bytes.Buffer)
	b.Grow(DatagramSize) // 2 + 2 + 512 = 516 max size

	// Increment the block number for this DATA packet
	d.Block++

	// Write the 2-byte DATA opcode (value = 3) in big-endian order
	if err := binary.Write(b, binary.BigEndian, OpData); err != nil {
		return nil, err
	}

	// Write the 2-byte block number
	if err := binary.Write(b, binary.BigEndian, d.Block); err != nil {
		return nil, err
	}

	// Copy up to 512 bytes from the payload into the buffer
	// io.CopyN will return io.EOF if less than 512 bytes are copied — which is OK (last block)
	if _, err := io.CopyN(b, d.Payload, BlockSize); err != nil && err != io.EOF {
		return nil, err
	}

	// Return the constructed byte slice
	return b.Bytes(), nil
}

// UnmarshalBinary parses a DATA packet from a byte slice.
// It extracts the block number and wraps the payload in a bytes.Reader.
func (d *Data) UnmarshalBinary(p []byte) error {
	// A valid DATA packet must be at least 4 bytes (opcode + block number)
	// and at most 516 bytes (full TFTP datagram)
	if l := len(p); l < 4 || l > DatagramSize {
		return errors.New("invalid Data")
	}

	var opcode OpCode

	// Read the first 2 bytes to determine the opcode
	if err := binary.Read(bytes.NewReader(p[:2]), binary.BigEndian, &opcode); err != nil || opcode != OpData {
		return errors.New("invalid DATA")
	}

	// Read the next 2 bytes for the block number
	if err := binary.Read(bytes.NewReader(p[2:4]), binary.BigEndian, &d.Block); err != nil {
		return errors.New("invalid DATA")
	}

	// Treat the remaining bytes as the data payload
	// We use a bytes.Buffer to implement io.Reader for the Payload field
	d.Payload = bytes.NewBuffer(p[4:])

	return nil
}

type Ack uint16

func (a Ack) MarshaBinary() ([]byte, error) {
	cap := 2 + 2

	b := new(bytes.Buffer)
	b.Grow(cap)

	err := binary.Write(b, binary.BigEndian, OpAck)
	if err != nil {
		return nil, err
	}
}
