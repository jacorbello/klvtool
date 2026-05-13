package stream

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jacorbello/klvtool/internal/model"
	"golang.org/x/net/ipv4"
)

// udpReadBufSize is the size of the per-datagram receive buffer. MPEG-TS
// over UDP traditionally ships 7×188 = 1316-byte payloads, but some
// encoders use other multiples or a full 1500-byte MTU. 2048 covers all
// real-world variants with headroom.
const udpReadBufSize = 2048

type udpSource struct {
	conn       *net.UDPConn
	pktConn    *ipv4.PacketConn // only set for multicast joins; lifecycle-bound to conn
	scheme     string
	remoteAddr string

	// buf carries the tail of the most recent datagram across Read calls so
	// a 188-byte scanner buffer can drain a 1316-byte datagram across
	// multiple reads without dropping bytes.
	buf []byte
}

func (u *udpSource) Read(p []byte) (int, error) {
	if len(u.buf) > 0 {
		n := copy(p, u.buf)
		u.buf = u.buf[n:]
		return n, nil
	}
	scratch := make([]byte, udpReadBufSize)
	n, _, err := u.conn.ReadFromUDP(scratch)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if n <= len(p) {
		return copy(p, scratch[:n]), nil
	}
	copied := copy(p, scratch[:n])
	u.buf = append(u.buf, scratch[copied:n]...)
	return copied, nil
}

func (u *udpSource) Close() error {
	// Closing the underlying conn also tears down the ipv4.PacketConn
	// wrapper; the multicast group membership is dropped by the OS when
	// the socket closes.
	return u.conn.Close()
}

func (u *udpSource) Scheme() string     { return u.scheme }
func (u *udpSource) RemoteAddr() string { return u.remoteAddr }

func openUDP(ctx context.Context, u *url.URL, opts Options) (Source, error) {
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("udp URL %q: %w", u.String(), err))
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("udp URL %q: port %q is not numeric", u.String(), portStr))
	}
	if port <= 0 || port > 65535 {
		return nil, model.InvalidUsage(fmt.Errorf("udp URL %q: port %d out of range", u.String(), port))
	}

	// Resolve the host into an IP. Empty host means "wildcard bind".
	var dstIP net.IP
	if host != "" {
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, model.BackendExecution(fmt.Errorf("resolve udp host %q: %w", host, err))
		}
		// Prefer the first IPv4 address; multicast is IPv4-only on this code
		// path. IPv6 multicast is a deferred concern.
		for _, ip := range ips {
			if v4 := ip.IP.To4(); v4 != nil {
				dstIP = v4
				break
			}
		}
		if dstIP == nil {
			return nil, model.InvalidUsage(fmt.Errorf("udp URL %q: no IPv4 address for host %q", u.String(), host))
		}
	}

	isMulticast := dstIP != nil && dstIP.IsMulticast()

	// For multicast, we bind to the group address; the OS routes group
	// traffic to the socket once we join. For unicast, we bind to the
	// wildcard and let the kernel deliver any datagrams addressed to our
	// port.
	bindIP := net.IPv4zero
	if isMulticast {
		bindIP = dstIP
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: port})
	if err != nil {
		return nil, model.BackendExecution(fmt.Errorf("listen udp %s:%d: %w", bindIP, port, err))
	}

	src := &udpSource{
		conn:       conn,
		scheme:     "udp",
		remoteAddr: u.Host,
	}

	if isMulticast {
		// Resolve the egress interface. Empty Iface means "ask the kernel
		// to pick" via the routing table.
		var iface *net.Interface
		ifaceName := opts.Iface
		if ifaceName == "" {
			// Allow the URL query param to specify iface too, matching
			// ffmpeg-style URIs.
			ifaceName = u.Query().Get("iface")
		}
		if ifaceName != "" {
			candidate, err := net.InterfaceByName(ifaceName)
			if err != nil {
				_ = conn.Close()
				return nil, model.InvalidUsage(fmt.Errorf("udp iface %q: %w", ifaceName, err))
			}
			iface = candidate
		}
		pc := ipv4.NewPacketConn(conn)
		if err := pc.JoinGroup(iface, &net.UDPAddr{IP: dstIP}); err != nil {
			_ = conn.Close()
			return nil, model.BackendExecution(fmt.Errorf("join multicast group %s: %w", dstIP, err))
		}
		src.pktConn = pc
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return src, nil
}
