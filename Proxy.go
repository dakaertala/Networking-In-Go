package main

import (
	"io"
	"net"
	"sync"
	"testing"
)

// proxyConn connects to two TCP endpoints (source and destination) and proxies data between them.
// It sets up bi-directional data transfer: any data received from the source is forwarded to the destination,
// and any data from the destination is sent back to the source.
func proxyConn(source, destination string) error {
	// Dial the source TCP address
	connSource, err := net.Dial("tcp", source)
	if err != nil {
		return err
	}
	defer connSource.Close() // Ensure source connection is closed when function exits

	// Dial the destination TCP address
	connDestination, err := net.Dial("tcp", destination)
	if err != nil {
		return err
	}
	defer connDestination.Close() // Ensure destination connection is closed

	// Start a goroutine to copy data from destination to source
	// (e.g., for responses coming back)
	go func() {
		_, _ = io.Copy(connSource, connDestination)
	}()

	// Start a goroutine to copy data from source to destination
	// (e.g., for requests going out)
	go func() {
		_, _ = io.Copy(connDestination, connSource)
	}()

	// NOTE: This function returns immediately without waiting for the copies to finish.
	// In real-world use, you might want to use synchronization (like `sync.WaitGroup`)
	// or a blocking operation to wait for these copies to finish.

	return nil
}

// proxy copies data from an io.Reader (`from`) to an io.Writer (`to`) with optional bi-directional support.
// If `from` also implements `io.Writer` and `to` implements `io.Reader`, it sets up reverse communication
// as well using a goroutine.
func proxy(from io.Reader, to io.Writer) error {
	// Check if `from` can also be written to (used for reverse copy)
	fromWriter, fromIsWriter := from.(io.Writer)
	// Check if `to` can also be read from (used for reverse copy)
	toReader, toIsReader := to.(io.Reader)

	if toIsReader && fromIsWriter {
		// If both directions are supported, copy data from `to` back to `from`
		go func() {
			_, _ = io.Copy(fromWriter, toReader)
		}()
	}

	// Main data transfer: copy from `from` to `to`
	_, err := io.Copy(to, from)
	return err
}

func TestProxy(t *testing.T) {
	var wg sync.WaitGroup

	// server listens for a "ping" message and responds with a
	// "pong" message. All other messages are echoed back to
	// the client
	server, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			conn, err := server.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				for {
					buf := make([]byte, 1024)
					n, err := c.Read(buf)
					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}

						return
					}

					switch msg := string(buf[:n]); msg {
					case "ping":
						_, err = c.Write([]byte("pong"))
					default:
						_, err = c.Write(buf[:n])
					}

					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}

						return
					}
				}
			}(conn)
		}
	}()

	// proxyServer proxies messages from client connections to the
	// destinationServer. Replies from the destinationServer are proxied
	// back to the clients.
	proxyServer, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			conn, err := proxyServer.Accept()
			if err != nil {
				return
			}

			go func(from net.Conn) {
				defer from.Close()

				to, err := net.Dial("tcp", server.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}

				defer to.Close()

				err = proxy(from, to)
				if err != nil && err != io.EOF {
					t.Error(err)
				}
			}(conn)
		}
	}()

	conn, err := net.Dial("tcp", proxyServer.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	msgs := []struct{ Message, Reply string }{
		{"ping", "pong"},
		{"pong", "pong"},
		{"echo", "echo"},
		{"ping", "pong"},
	}

	for i, m := range msgs {
		_, err = conn.Write([]byte(m.Message))
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		actual := string(buf[:n])
		t.Logf("%q -> proxy -> %q", m.Message, actual)
		if actual != m.Reply {
			t.Errorf("%d: expected reply: %q; actual: %q",
				i, m.Reply, actual)
		}
	}

	_ = conn.Close()
	_ = proxyServer.Close()
	_ = server.Close()

	wg.Wait()
}
