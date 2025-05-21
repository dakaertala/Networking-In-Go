package main

import (
	"context"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestDial(t *testing.T) {
	// Create a listener on random port
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// Channel to signal when goroutines are done
	// Ensuring clean exit and test completion
	done := make(chan struct{})

	// Server goroutine
	// Handles accepting connections
	go func() {
		// Sending a signal when the function exits
		// Channel of empty structs is for signaling, it
		// uses no memory
		defer func() { done <- struct{}{} }()

		// Infinite loop
		for {
			// Accepting new connections in a loop
			conn, err := listener.Accept()
			if err != nil {
				t.Log(err)
				return
			}

			// Each connection is handled in its own goroutine
			go func(c net.Conn) {
				// Sending a signal when the function exits
				defer func() {
					c.Close()
					done <- struct{}{}
				}()

				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}
						// In case of EOF
						// which means connection closed by client
						// return normally
						return
					}

					t.Logf("received: %q", buf[:n])
				}
			}(conn)
		}
	}()

	// Client code
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// Closes the connection, which trigger EOF
	conn.Close()
	// Waiting for connection handler to exit
	<-done
	listener.Close()
	// Waiting for listener to exit
	<-done
}

func TestDialContext(t *testing.T) {
	dl := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), dl)
	defer cancel()

	var d net.Dialer
	d.Control = func(_, _ string, _ syscall.RawConn) error {
		// Sleep long enough to reach the context's deadline
		time.Sleep(5*time.Second + time.Millisecond)
		return nil
	}

	conn, err := d.DialContext(ctx, "tcp", "10.0.0.0:80")
	if err == nil {
		conn.Close()
		t.Fatal("connection did not time out")
	}

	nErr, ok := err.(net.Error)
	if !ok {
		t.Error(err)
	} else {
		if !nErr.Timeout() {
			t.Errorf("error is not a timeout: %v", err)
		}
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("expected deadline exceeded; actual: %v", ctx.Err())
	}
}

// TestDialContextCancel checks that net.Dialer.DialContext respects context cancellation.
// It simulates a slow connection and cancels the context before the dial completes.
func TestDialContextCancel(t *testing.T) {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Channel used to wait for the goroutine to finish
	sync := make(chan struct{})

	// Server side logic
	go func() {
		// Notify the main thread when done
		defer func() { sync <- struct{}{} }()

		// Create a custom dialer
		var d net.Dialer

		// Control hook simulates a delay before dialing completes
		d.Control = func(_, _ string, _ syscall.RawConn) error {
			time.Sleep(time.Second) // Simulate a 1-second setup delay
			return nil              // No real error — just slow setup
		}

		// Attempt to dial with the cancellable context
		conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")
		if err != nil {
			// Expected: dialing fails due to context cancellation
			t.Log(err)
			return
		}

		// If dialing unexpectedly succeeds, fail the test
		conn.Close()
		t.Error("connection did not timeout")
	}()

	// Cancel the context almost immediately after starting the dial
	cancel()

	// Wait for the goroutine to complete
	<-sync

	// Check that the context reports it was canceled
	if ctx.Err() != context.Canceled {
		t.Errorf("expected canceled context; actual: %q", ctx.Err())
	}
}
