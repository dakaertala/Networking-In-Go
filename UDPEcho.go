package main

import (
	"context"
	"fmt"
	"net"
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
