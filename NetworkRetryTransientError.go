package main

import (
	"errors"
	"log"
	"net"
	"syscall"
	"time"
)

func SendWithRetry(conn net.Conn, data []byte) error {
	var (
		err        error
		n          int
		maxRetries = 7
	)

	for i := 0; i < maxRetries; i++ {
		n, err = conn.Write(data)
		if err != nil {
			// Retry only on known transient errors
			if isTransientError(err) {
				log.Printf("transient error on write (attempt %d/%d): %v", i+1, maxRetries, err)
				time.Sleep(10 * time.Second)
				continue
			}

			// Not a retryable error
			return err
		}
		// Write was successful
		log.Printf("wrote %d bytes to %s\n", n, conn.RemoteAddr())
		return nil
	}

	// All retries failed
	return errors.New("temporary write failure threshold exceeded")
}

// Checks if the error is a retryable transient network error
func isTransientError(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE)
}
