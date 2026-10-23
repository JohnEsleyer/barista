package jnet

import (
	"fmt"
	"net"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

// SocketAddress is the abstract base for socket addresses.
type SocketAddress struct {
	lang.Object
}

// InetSocketAddress is the native shim for java.net.InetSocketAddress.
type InetSocketAddress struct {
	SocketAddress
	Host string
	Port int32
}

// NewInetSocketAddress creates an InetSocketAddress.
func NewInetSocketAddress(port int32) *InetSocketAddress {
	return &InetSocketAddress{Host: "127.0.0.1", Port: port}
}

// Init initializes an InetSocketAddress from constructor arguments.
func (addr *InetSocketAddress) Init(args ...any) *InetSocketAddress {
	if addr == nil {
		return addr
	}
	if len(args) == 1 {
		if p, ok := args[0].(int32); ok {
			addr.Port = p
			addr.Host = "127.0.0.1"
		}
	} else if len(args) >= 2 {
		if h, ok := args[0].(*lang.String); ok && h != nil {
			addr.Host = h.GoString()
		} else if s, ok := args[0].(string); ok {
			addr.Host = s
		}
		if p, ok := args[1].(int32); ok {
			addr.Port = p
		}
	}
	return addr
}

// GetPort returns the port number.
func (addr *InetSocketAddress) GetPort() int32 {
	if addr == nil {
		return 0
	}
	return addr.Port
}

// GetHostName returns the host name.
func (addr *InetSocketAddress) GetHostName() *lang.String {
	if addr == nil {
		return lang.NewString("")
	}
	return lang.NewString(addr.Host)
}

// ToString renders the socket address.
func (addr *InetSocketAddress) ToString() *lang.String {
	if addr == nil {
		return lang.NewString("null")
	}
	return lang.NewString(fmt.Sprintf("%s:%d", addr.Host, addr.Port))
}

// ServerSocket is the native shim for java.net.ServerSocket.
type ServerSocket struct {
	lang.Object
	listener net.Listener
	closed   bool
}

// Init creates and binds the ServerSocket.
func (s *ServerSocket) Init(args ...any) *ServerSocket {
	if s == nil {
		return s
	}
	port := int32(0)
	if len(args) > 0 {
		if p, ok := args[0].(int32); ok {
			port = p
		}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	s.listener = ln
	return s
}

// Accept accepts an incoming connection and returns a Socket.
func (s *ServerSocket) Accept() *Socket {
	if s == nil || s.listener == nil || s.closed {
		panic(lang.NewIOException(lang.NewString("server socket is closed")))
	}
	conn, err := s.listener.Accept()
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	return &Socket{conn: conn}
}

// GetLocalPort returns the bound port.
func (s *ServerSocket) GetLocalPort() int32 {
	if s == nil || s.listener == nil {
		return -1
	}
	if tcpAddr, ok := s.listener.Addr().(*net.TCPAddr); ok {
		return int32(tcpAddr.Port)
	}
	return -1
}

// Close closes the server socket.
func (s *ServerSocket) Close() {
	if s != nil && s.listener != nil && !s.closed {
		s.closed = true
		_ = s.listener.Close()
	}
}

// IsClosed reports whether the socket is closed.
func (s *ServerSocket) IsClosed() bool {
	return s == nil || s.closed
}

// Socket is the native shim for java.net.Socket.
type Socket struct {
	lang.Object
	conn   net.Conn
	in     *InputStream
	out    *OutputStream
	closed bool
}

// Init connects to the target host and port.
func (s *Socket) Init(args ...any) *Socket {
	if s == nil || len(args) < 2 {
		return s
	}
	host := "127.0.0.1"
	if h, ok := args[0].(*lang.String); ok && h != nil {
		host = h.GoString()
	} else if hs, ok := args[0].(string); ok {
		host = hs
	}
	port := args[1].(int32)
	conn, err := net.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	s.conn = conn
	return s
}

// GetInputStream returns an InputStream reading from this connection.
func (s *Socket) GetInputStream() *InputStream {
	if s == nil || s.conn == nil {
		panic(lang.NewIOException(lang.NewString("socket not connected")))
	}
	if s.in == nil {
		s.in = NewInputStream(s.conn)
	}
	return s.in
}

// GetOutputStream returns an OutputStream writing to this connection.
func (s *Socket) GetOutputStream() *OutputStream {
	if s == nil || s.conn == nil {
		panic(lang.NewIOException(lang.NewString("socket not connected")))
	}
	if s.out == nil {
		s.out = NewOutputStream(s.conn)
	}
	return s.out
}

// GetPort returns the remote port.
func (s *Socket) GetPort() int32 {
	if s == nil || s.conn == nil {
		return 0
	}
	if tcp, ok := s.conn.RemoteAddr().(*net.TCPAddr); ok {
		return int32(tcp.Port)
	}
	return 0
}

// GetLocalPort returns the local port.
func (s *Socket) GetLocalPort() int32 {
	if s == nil || s.conn == nil {
		return 0
	}
	if tcp, ok := s.conn.LocalAddr().(*net.TCPAddr); ok {
		return int32(tcp.Port)
	}
	return 0
}

// Close closes the socket connection.
func (s *Socket) Close() {
	if s != nil && s.conn != nil && !s.closed {
		s.closed = true
		_ = s.conn.Close()
	}
}

// IsClosed reports whether the socket is closed.
func (s *Socket) IsClosed() bool {
	return s == nil || s.closed
}

// IsConnected reports whether the socket has an active connection.
func (s *Socket) IsConnected() bool {
	return s != nil && s.conn != nil && !s.closed
}
