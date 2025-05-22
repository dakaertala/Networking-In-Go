package main

import (
	"crypto/rand"
	"io"
	"net"
	"testing"
)

func TestReadIntoBuffer(t *testing.T) {
	// Create a 16MB (1 << 24) payload filled with random data
	payload := make([]byte, 1<<24)

	_, err := rand.Read(payload)
	if err != nil {
		// Fail the test if random data generation fails
		t.Fatal(err)
	}

	// Start a TCP listener on a local address with an automatically assigned port
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		// Fail the test if listener creation fails
		t.Fatal(err)
	}

	// Server side
	// Start a goroutine to accept a single connection
	// and write the payload to it
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			// Log error if accepting the connection fails
			t.Log(err)
			return
		}
		defer conn.Close()

		// Write the entire payload to the connection
		_, err = conn.Write(payload)
		if err != nil {
			// Log error if writing the payload fails
			t.Error(err)
		}
	}()

	// Dial the listener to simulate a client
	// receiving the payload
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		// Fail the test if connection fails
		t.Fatal(err)
	}

	// Create a 512KB (1 << 19) buffer for reading
	// from the connection
	buf := make([]byte, 1<<19)

	// Continuously read from the connection
	// into the buffer until EOF
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Log any error other than EOF
				t.Error(err)
			}
			// Exit the loop on EOF error
			break
		}

		// Log how many bytes were read in this iteration
		t.Logf("read %d bytes", n)
	}

	// Close the connection when done
	conn.Close()
}
