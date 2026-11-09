//go:build !linux

package main

import (
	"errors"
	"net"
)

// peerUID is unsupported off Linux: kuberexctl serve runs only in the pod.
func peerUID(net.Conn) (uint32, error) {
	return 0, errors.New("peer credentials are read only on Linux")
}
