package main

import (
	"net"
	"syscall"
	"testing"
	"time"
)

// Without Context
// DialTimeout demonstrates a custom Dialer that always simulates a connection timeout error.
// It does NOT actually try to establish a real network connection.
// Instead, it returns a controlled DNS error with timeout flags.
func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{
		// A hook that runs just before the connection is established
		Control: func(_, addr string, _ syscall.RawConn) error {
			// Instead of continuing we return a fake DNS error
			return &net.DNSError{
				Err:         "connection timed out",
				Name:        addr,
				Server:      "127.0.0.1",
				IsTimeout:   true,
				IsTemporary: true,
			}
		},
		// This sets the overall timeout on the dialer
		// Won’t matter because the dial fails immediately in the Control hook
		Timeout: timeout,
	}
	return d.Dial(network, address)
}

func TestDialTimeout(t *testing.T) {
	c, err := DialTimeout("tcp", "10.0.0.1:http", 5*time.Second)
	if err == nil {
		c.Close()
		t.Fatal("connection did not time out")
	}

	nErr, ok := err.(net.Error)

	if !ok {
		t.Fatal(err)
	}

	if !nErr.Timeout() {
		t.Fatal("error is not timeout")
	}
}
