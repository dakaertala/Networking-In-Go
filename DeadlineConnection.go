package main

import (
	"io"
	"net"
	"testing"
	"time"
)

// Useful to not to rely on TCP FIN packets.

// The go routine acts as a server, and the client is the main function execution thread

// When you read data from the remote node, you push the deadline forward. The
// remote node sends more data, and you push the deadline forward again,
// and so on. If you don’t hear from the remote node in the allotted time, you
// can assume that either the remote node is gone and you never received its
// FIN or that it is idle.
func TestDeadline(t *testing.T) {
	sync := make(chan struct{}) // Channel used to synchronize between goroutines

	// Start listening on a random TCP port on loopback address
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// Start server logic in a goroutine
	go func() {
		// Accept an incoming connection
		// Waiting for the client to connect
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		// Close everything up on exit out of the goroutine
		// to finish that everything is done
		defer func() {
			conn.Close()
			close(sync)
		}()

		// Set a 5 second read deadline. If no data is received within 5 seconds, read
		// or write will fail with a timeout.
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		buf := make([]byte, 1) // 1-byte buffer for reading data
		// This call waits up to 5 seconds and then fails with timeout
		_, err = conn.Read(buf) // Try reading from the client. Expect a timeout.

		// Assert that the error is timeout
		nErr, ok := err.(net.Error)
		if !ok || !nErr.Timeout() {
			t.Errorf("expected timeout error; actual: %v", err)
		}

		// Notify the main test goroutine that it's time to send data
		sync <- struct{}{}

		// Extend the deadline again to read more data
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		// Try reading again, this time the client will send data.
		_, err = conn.Read(buf)
		if err != nil {
			t.Error(err)
		}
	}()

	// Main test goroutine connect to the server
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Wait until the server signals it hit the timeout
	<-sync

	// Send one byte to the server to satisfy its second read.
	_, err = conn.Write([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to read response; expecting connection close (EOF)
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != io.EOF {
		t.Errorf("expected server termination; actual: %v", err)
	}
}
