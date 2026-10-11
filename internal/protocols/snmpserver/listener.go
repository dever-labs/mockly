package snmpserver

import (
	"net"
)

// udpListener is a minimal, single-owner UDP listener. Unlike GoSNMPServer's
// equivalent (the dependency this package used to use — see issue #113),
// it never reassigns its net.UDPConn field after construction: Shutdown
// closes the connection but does not nil the field, so a concurrent
// in-flight ReadFromUDP call simply observes the close as a normal
// net.ErrClosed read error. This avoids the data race inherent in clearing
// the field from one goroutine while another goroutine reads it, without
// needing any additional synchronization.
type udpListener struct {
	conn *net.UDPConn
}

// listenUDP opens a UDP socket at address.
func listenUDP(address string) (*udpListener, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &udpListener{conn: conn}, nil
}

// Addr returns the local address the listener is bound to.
func (l *udpListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// Close shuts down the listener, causing any in-flight or future ReadFrom
// call to return an error.
func (l *udpListener) Close() error {
	return l.conn.Close()
}

// ReadFrom reads the next datagram into buf, returning its length and the
// sender's address.
func (l *udpListener) ReadFrom(buf []byte) (int, *net.UDPAddr, error) {
	return l.conn.ReadFromUDP(buf)
}

// WriteTo sends buf to addr.
func (l *udpListener) WriteTo(buf []byte, addr *net.UDPAddr) error {
	_, err := l.conn.WriteToUDP(buf, addr)
	return err
}
