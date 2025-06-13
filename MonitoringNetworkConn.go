package main

import (
	"io"
	"log"
	"net"
	"os"
)

// Network Traffic Monitor and Echo Server
//
// This program demonstrates how to monitor (log) network traffic flowing in both
// directions while acting as an echo server. Here's what it does:
//
// 1. Sets up a TCP server on localhost that listens for incoming connections
// 2. Monitors incoming data: Uses io.TeeReader to simultaneously read data from
//    the connection and log what was received
// 3. Echoes data back: Uses io.MultiWriter to simultaneously send the received
//    data back to the client and log what was sent
// 4. Logs everything: The Monitor struct implements io.Writer so it can be
//    plugged into Go's io operations, logging all data with a "monitor: " prefix
//
// Example usage with telnet:
// - Client sends "Hello" → server logs "monitor: Hello"
// - Server echoes "Hello" back → logs "monitor: Hello" again
// - Both incoming and outgoing traffic are monitored
//
// This pattern creates a transparent network proxy/monitor - data flows through
// normally, but everything gets logged for debugging or monitoring purposes.

// Monitor wraps a logger and implements io.Writer interface
// This allows it to be used as a destination for monitoring network traffic
type Monitor struct {
	*log.Logger
}

// Write implements the io.Writer interface for Monitor
// It logs the data being written and returns the length to satisfy the interface
func (m *Monitor) Write(p []byte) (int, error) {
	// Return the full length of the data and log it
	// Using Output(2, ...) skips 2 stack frames to show the actual caller
	return len(p), m.Output(2, string(p))
}

func ExampleMonitor() {
	// Create a new Monitor with a logger that prefixes output with "monitor: "
	monitor := &Monitor{Logger: log.New(os.Stdout, "monitor: ", 0)}

	// Create a TCP listener on localhost with a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		monitor.Fatal(err)
	}

	// Channel to signal when the goroutine is done
	done := make(chan struct{})

	// Start a goroutine to handle incoming connections
	go func() {
		// Ensure the done channel is closed when the goroutine exits
		defer close(done)

		// Accept a single incoming connection
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Ensure the connection is closed when done
		defer conn.Close()

		// Buffer to read incoming data
		b := make([]byte, 1024)

		// Create a TeeReader that reads from the connection and simultaneously
		// writes the data to the monitor (for logging incoming data)
		r := io.TeeReader(conn, monitor)

		// Read data from the connection (this will also log it via TeeReader)
		n, err := r.Read(b)
		if err != nil && err != io.EOF {
			monitor.Println(err)
			return
		}

		// Create a MultiWriter that writes to both the connection and monitor
		// This allows us to echo the message back while also logging it
		w := io.MultiWriter(conn, monitor)

		// Echo the received message back to the client and log it
		_, err = w.Write(b[:n])
		if err != nil && err != io.EOF {
			monitor.Println(err)
			return
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		monitor.Fatal(err)
	}

	_, err = conn.Write([]byte("Test\n"))
	if err != nil {
		monitor.Fatal()
	}

	_ = conn.Close()
	<-done
}
