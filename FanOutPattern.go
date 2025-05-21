package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// Fan-out pattern is especially useful in distributed systems
// It allows you to parallelize work, increase throughput, or
// race for the fastest result, depending on how it's used.

// Fan-out is often used with fan-in, all the results of goroutines
// are combined into one

// ! If the task doesn't take a long time to compute or wait
// ! on input/output, then using concurrency may not help.
// ! And can actually make things more confusing and error-prone

// Fan-out pattern is where multiple goroutines (dialers) attempt
// to establish a TCP connection concurrently but only one
// should succeed, and the rest should stop as soon as the result
// is obtained and context is canceled.
func TestDialContextCancelFanOut(t *testing.T) {
	// Create a context with a deadline 10 seconds from now
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(10*time.Second),
	)

	// Start a tcp listener on a random available port on localhost
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// Simulation of a server side
	// Accept one connection then end it immediately
	go func() {
		// Only accepting a single connection
		conn, err := listener.Accept()
		if err == nil {
			// Close the connection immediately after accepting
			conn.Close()
		}
	}()

	// The dial function attempts to dial the listener and sends the ID of the successful dialer
	dial := func(ctx context.Context, address string, response chan int, id int, wg *sync.WaitGroup) {
		defer wg.Done()

		var d net.Dialer

		// Attempt to dial using the provided context
		c, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			// If dialing fails (e.g. connection refused or context canceled), just return
			return
		}
		// Close the connection immediately upon success
		c.Close()

		// Try to send the ID to the response channel unless the context is already canceled
		select {
		case <-ctx.Done():
			// Context canceled before sending ID, do nothing
		case response <- id: // otherwise send id to response
			// Successfully sent the ID of the dialer that connected
		}
	}

	// Channel to receive the ID of the successful dialer
	res := make(chan int)
	// WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Launch 10 concurrent dialers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go dial(ctx, listener.Addr().String(), res, i+1, &wg)
	}

	// Wait for the first dialer to report success
	response := <-res

	// Cancel the context to signal all other dialers to stop
	cancel()

	// Wait for all dialers to complete
	wg.Wait()

	// Close the response channel
	close(res)

	// Check if the context was indeed canceled
	if ctx.Err() != context.Canceled {
		t.Errorf("expected canceled context; actual: %s", ctx.Err())
	}

	// Log the ID of the dialer that succeeded first
	t.Logf("dialer %d retrieved the resource", response)
}
