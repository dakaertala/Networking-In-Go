package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
)

// echoServerUDP starts a simple UDP echo server.
// It binds to the provided address (e.g., ":12345") and starts listening for UDP packets.
// Whenever it receives a packet, it echoes the same data back to the sender.
// The server runs asynchronously in a goroutine and is stopped when the context is canceled.
//
// Parameters:
// - ctx: a context that can be used to cancel the server (for graceful shutdown).
// - addr: the local address to bind the server to (can be IP:port or just port).
//
// Returns:
// - net.Addr: the actual address the server is bound to (useful if addr was ":0").
// - error: if binding fails, returns a wrapped error; otherwise, returns nil.
func echoServerUDP(ctx context.Context, addr string) (net.Addr, error) {
	// Try to bind to the given UDP address (e.g., ":0" for any available port)
	s, err := net.ListenPacket("udp", addr)
	if err != nil {
		// If binding fails, return a formatted error
		return nil, fmt.Errorf("binding to udp %s: %w", addr, err)
	}

	// Start the server logic in a separate goroutine to avoid blocking the caller
	go func() {
		// Start another goroutine whose only job is to watch for context cancellation
		// and close the UDP socket when it's done.
		// This ensures the server exits cleanly when the parent context is canceled.
		go func() {
			// Wait for cancellation signal
			<-ctx.Done()
			// Close the socket to unblock ReadFrom/WriteTo
			_ = s.Close()
		}()

		// Allocate a fixed-size buffer to read incoming UDP datagrams
		buf := make([]byte, 1024)

		for {
			// Block and wait for the next incoming UDP packet
			n, clientAddr, err := s.ReadFrom(buf)
			if err != nil {
				// Exit the loop on error (likely caused by socket closure)
				return
			}

			// Echo the received data back to the client using the same connection
			_, err = s.WriteTo(buf[:n], clientAddr)
			if err != nil {
				// If writing fails (e.g., network error), exit the loop
				return
			}
		}
	}()

	// Return the actual address we're bound to (useful when using ":0")
	return s.LocalAddr(), err
}

// Properly verifies that the echo server properly receives and replies to a UDP packet
func TestEchoServerUDP(t *testing.T) {
	// Create a cancellable context to create a server lifecycle
	ctx, cancel := context.WithCancel(context.Background())

	// Start the echo server on a random available UDP port
	// on localhost
	serverAddr, err := echoServerUDP(ctx, "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	// Ensure that the server is shutdown at the end of the test
	defer cancel()

	// Open a UDP client socket at an available port
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	// Close the client socket when the test completes
	defer func() { _ = client.Close() }()

	// Message to send to the server
	msg := []byte("ping")

	// Send the message to the server's address
	_, err = client.WriteTo(msg, serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate the buffer to receive the server's reply
	buf := make([]byte, 1024)

	// Wait for a reply from the server
	n, addr, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the response comes from the server
	if addr.String() != serverAddr.String() {
		t.Fatalf("received reply from %q instead of %q", addr, serverAddr)
	}

	// Ensure the message content matches what was sent
	if !bytes.Equal(msg, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", msg, buf[:n])
	}
}

// Tests that the client socket receives messages from
// multiple sources and distinguishes between them correctly
func TestListenPacketUDP(t *testing.T) {
	// Create cancellable context to controll the server
	ctx, cancel := context.WithCancel(context.Background())

	// Start the udp echo server
	// It will echo back any packet it receives to the sender
	serverAddr, err := echoServerUDP(ctx, "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	// Create the client that will receive the message from
	// the echo server
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	// Create an "interloper" UDP socket — a second, unrelated sender
	// that will send a fake message directly to the client.
	interloper, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// The interloper prepares a message to send to the client
	interrupt := []byte("pardon me")
	// Interloper sends the message directly to the client's address.
	n, err := interloper.WriteTo(interrupt, client.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}

	// Interloper has served its purpose, so it is closed now.
	_ = interloper.Close()

	// Ensure that the number of bytes reported as written
	// matches the message length.
	if l := len(interrupt); l != n {
		t.Fatalf("wrote %d bytes of %d", n, l)
	}

	// Prepare a second message — this one is a
	// proper "ping" to the echo server.
	ping := []byte("ping")

	// Client sends the ping message to the echo server,
	// which should echo the message back to the client.
	_, err = client.WriteTo(ping, serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate a buffer to receive incoming UDP packets
	buf := make([]byte, 1024)

	// First read: Expecting the interrupt message from the interloper.
	n, addr, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the contents of the received message.
	if !bytes.Equal(interrupt, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", interrupt, buf[:n])
	}

	// Validate the source address: it should match the interloper.
	if addr.String() != interloper.LocalAddr().String() {
		t.Errorf("expected message from %q; actual sender %q", interloper.LocalAddr(), addr)
	}

	// Second read:  Expecting the echoed "ping" message from the server.
	n, addr, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the contents of the echoed ping message.
	if !bytes.Equal(ping, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", ping, buf[:n])
	}

	// Validate that the echo came from the correct server address.
	if addr.String() != serverAddr.String() {
		t.Errorf("expected message from %q; actual sender is %q",
			serverAddr, addr)
	}
}
