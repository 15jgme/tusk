package main

import (
	"fmt"
	"net"
)

func isHostPortAvailable(port uint16) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Port is in use, return unavailable
		return false
	}
	err = ln.Close()
	if err != nil {
		// Oops cannot close the connection, bad news bears
		panic(err)
	}

	// Port was detected as available
	return true

}
