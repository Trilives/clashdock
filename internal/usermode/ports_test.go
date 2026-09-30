package usermode

import (
	"net"
	"reflect"
	"testing"
)

func TestBusyPortsDetectsListeningPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on loopback: %v", err)
	}
	busyPort := ln.Addr().(*net.TCPAddr).Port

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	got := BusyPorts(freePort, busyPort)
	ln.Close()
	if !reflect.DeepEqual(got, []int{busyPort}) {
		t.Fatalf("BusyPorts(%d free, %d busy) = %v", freePort, busyPort, got)
	}
}
