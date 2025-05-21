package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// In case of long running network connections
// that may experience extended idle periods
// we utilize heartbeats.
// This is something you have on your nodes,
// sending beats to the control plane.

const defaultPingInterval = 30 * time.Second

// Function that pings a network at a regular interval
// - ctx: Context for cancellation (e.g., to stop the pinger)
// - w: io.Writer to send "ping" messages to
// - reset: Channel to receive new ping intervals
func Pinger(ctx context.Context, w io.Writer, reset <-chan time.Duration) {
	var interval time.Duration // Stores the current ping interval

	// Initial interval setup: check if a new interval is
	// available on the reset channel
	select {
	case <-ctx.Done(): // If context has already been canceled, exit immediately
		return
	case interval = <-reset: // Read new interval from reset channel if available
	default: // No interval provided, do nothing
	}

	// If no valid interval was provided
	// (or interval <= 0), use defaultPingInterval
	if interval <= 0 {
		interval = defaultPingInterval
	}

	// Create a timer that fires after the specified interval
	timer := time.NewTimer(interval)
	// Ensure that the timer is stopped and drained on exit
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()

	// Main loop
	for {
		select {
		case <-ctx.Done(): // Context canceled, exit the function
			return
		// New interval received on reset channel
		case newInterval := <-reset:
			// Stop the current timer and drain
			if !timer.Stop() {
				<-timer.C
			}
			// Update interval if the new one is valid (> 0)
			if newInterval > 0 {
				interval = newInterval
			}
		// Timer fired, time to send a ping
		case <-timer.C:
			// Write "ping" to the writer
			if _, err := w.Write([]byte("ping")); err != nil {
				// track and act on consecutive timeouts here
				// If writing fails, exit
				// (could track consecutive errors in a real app)
				return
			}
		}

		// Reset the timer to fire again after the current interval
		_ = timer.Reset(interval)
	}
}

// ExamplePinger demonstrates the Pinger function in action
func ExamplePinger() {
	// Create a cancellable context to control the Pinger’s lifecycle
	ctx, cancel := context.WithCancel(context.Background())

	// Create a pipe for simulating network communication (reader and writer ends)
	r, w := io.Pipe()

	// Channel to signal when Pinger is done
	done := make(chan struct{})

	// Channel to send new ping intervals to Pinger
	resetTimer := make(chan time.Duration, 1)

	// Initialize with a 1-second interval
	resetTimer <- time.Second

	// Run Pinger in a separate goroutine
	go func() {
		Pinger(ctx, w, resetTimer)
		close(done) // Signal completion when Pinger exits
	}()

	// receivePing simulates receiving and processing pings
	// Parameters:
	// - d: Duration to send to resetTimer (new interval); negative means no reset
	// - r: io.Reader to read pings from
	receivePing := func(d time.Duration, r io.Reader) {
		// If duration is non-negative, send it to reset the ping interval
		if d >= 0 {
			fmt.Printf("resetting timer (%s)\n", d)
			resetTimer <- d
		}

		now := time.Now()
		buf := make([]byte, 1024)
		n, err := r.Read(buf)

		if err != nil {
			fmt.Println(err)
		}

		fmt.Printf("received %q (%s)\n",
			buf[:n], time.Since(now).Round(100*time.Millisecond))
	}

	// Test Pinger with a sequence of interval resets
	for i, v := range []int64{0, 200, 300, 0, -1, -1, -1} {
		fmt.Printf("Run %d:\n", i+1)
		// Call receivePing with millisecond durations (negative means no reset)
		receivePing(time.Duration(v)*time.Millisecond, r)
	}

	// Cancel the context to stop Pinger
	cancel()
	// Wait for Pinger to finish
	<-done
}

// Each side of a network connection could use a
// Pinger to advance its deadline if the other
// side becomes idle, whereas the previous examples
// showed only a single side using a Pinger.
// When either node receives data on the
// network connection, its ping timer should
// reset to stop the delivery of an unnecessary ping

// So on the server side every time we read from
// connection stream we reset ping timer and connection deadline.
// Client side we read what the pinger sends, we send a pong,
// which reset the ping timer which delays its firing.
// Then we receive pings again. And then we wait for
// everything to finish.
func TestPingerAdvanceDeadline(t *testing.T) {
	// Create a channel to signal when the server
	// goroutine completes
	done := make(chan struct{})

	// Start a TCP listener on localhost with an OS-assigned port.
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		// Fail the test if the listener cannot be created.
		t.Fatal(err)
	}

	// Record the start time for logging and calculating
	// test duration.
	begin := time.Now()

	// Launch a goroutine to handle the server side of the connection.
	go func() {
		// Ensure the done channel is closed when the goroutine exits.
		defer func() { close(done) }()

		// Accept a client connection
		conn, err := listener.Accept()
		if err != nil {
			// Log the error and exit the goroutine
			// (non-fatal, as it's in a goroutine).
			t.Log(err)
			return
		}

		// Create a context to control the
		// Pinger goroutine and ensure cleanup.
		ctx, cancel := context.WithCancel(context.Background())
		// Cancel the context and close the connection
		// when the goroutine exits.
		defer func() {
			cancel()
			conn.Close()
		}()

		// Create a buffered channel to send ping
		// intervals to the Pinger.
		resetTimer := make(chan time.Duration, 1)
		// Initialize Pinger with a 1-second ping interval.
		resetTimer <- time.Second

		// Start the Pinger goroutine to send
		// periodic pings over the connection.
		go Pinger(ctx, conn, resetTimer)

		// Set an initial 5-second deadline for connection
		// reads/writes.
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		// Create a 1KB buffer to read incoming data.
		buf := make([]byte, 1024)
		// Continuously read from the connection.
		for {
			n, err := conn.Read(buf)
			if err != nil {
				// Exit the loop on any read
				// error (e.g., EOF or deadline).
				return
			}
			// Log the time since the test began and the received data.
			t.Logf("[%s] %s",
				time.Since(begin).Truncate(time.Second), buf[:n])

			// Send 0 to resetTimer to reset or pause the Pinger's timer.
			resetTimer <- 0

			// Reset the connection deadline to 5 seconds from now.
			err = conn.SetDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				// Log the error and exit the goroutine (non-fatal).
				t.Error(err)
				return
			}
		}
	}()

	// Connect to the server as a client using the listener's address.
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		// Fail the test if the client cannot connect.
		t.Fatal(err)
	}
	// Ensure the client connection is closed when the test ends.
	defer conn.Close()

	// Create a 1KB buffer for the client to read pings.
	buf := make([]byte, 1024)
	// Read up to 4 pings from the server.
	for i := 0; i < 4; i++ {
		n, err := conn.Read(buf)
		if err != nil {
			// Fail the test if reading a ping fails.
			t.Fatal(err)
		}
		// Log the time since the test began and the received ping data.
		t.Logf("[%s] %s", time.Since(begin).Truncate(time.Second), buf[:n])
	}

	// Send "PONG!!!" to the server to reset its ping timer.
	_, err = conn.Write([]byte("PONG!!!")) // should reset the ping timer
	if err != nil {
		// Fail the test if writing to the server fails.
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ { // read up to four more pings
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
			break
		}
		t.Logf("[%s] %s", time.Since(begin).Truncate(time.Second), buf[:n])
	}

	// Wait for the server goroutine to complete.
	<-done
	// Calculate the total test duration, truncated to seconds.
	end := time.Since(begin).Truncate(time.Second)
	t.Logf("[%s] done", end)
	// Verify that the test duration is exactly 9 seconds.
	if end != 9*time.Second {
		t.Fatalf("expected EOF at 9 seconds; actual %s", end)
	}
}
