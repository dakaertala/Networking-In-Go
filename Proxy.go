package main

import (
	"io"
	"net"
)

func proxyConn(source, destination string) error {
	connSource, err := net.Dial("tcp", source)
	if err != nil {
		return err
	}
	defer connSource.Close()

	connDestination, err := net.Dial("tcp", destination)
	if err != nil {
		return err
	}
	defer connDestination.Close()

	// connDestination sends data to connSource
	go func() { _, _ = io.Copy(connSource, connDestination) }()

	// connSource sends data to connDestination
	go func() { _, _ = io.Copy(connDestination, connSource) }()

	return nil
}
