package main

import (
	"bufio"
	"net"
	"reflect"
	"testing"
)

const payload = "The bigger the interface, the weaker the abstraction."

func TestScanner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// Start a goroutine to act as a server
	go func() {
		// Accept a single incoming connection
		conn, err := listener.Accept()
		if err != nil {
			// Error logs the error but doesn't
			// stop the test
			t.Error(err)
			return
		}
		// Ensuring connection is closed on
		// goroutine exit
		defer conn.Close()

		// Write the payload to the client
		_, err = conn.Write([]byte(payload))
		if err != nil {
			t.Error(err)
		}
	}()

	// Act as a client and connect to our own server
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Setup a scanner to read from the connection
	scanner := bufio.NewScanner(conn)
	// Configure scanner to split input by whitespace
	scanner.Split(bufio.ScanWords)

	// Read all words from the connection
	var words []string

	// Scan() returns true while there are more
	// tokens to read, it returns false when EOF
	// or error occurs
	for scanner.Scan() {
		words = append(words, scanner.Text())
	}

	// Check if scanner encountered any errors
	// during scanning
	err = scanner.Err()
	if err != nil {
		t.Error(err)
	}

	expected := []string{"The", "bigger", "the", "interface,", "the",
		"weaker", "the", "abstraction."}

	// Use reflect.DeepEqual to compare slices element by element
	// This is more reliable than comparing slice headers directly
	if !reflect.DeepEqual(words, expected) {
		t.Fatal("inaccurate scanned word list")
	}

	// Log the successful result for debugging/verification purposes
	t.Logf("Scanned words: %#v", words)
}
