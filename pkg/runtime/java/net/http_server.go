package jnet

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

// HttpHandler is the interface matching com.sun.net.httpserver.HttpHandler.
type HttpHandler interface {
	Handle(exchange *HttpExchange)
}

// Headers is the native shim for com.sun.net.httpserver.Headers.
type Headers struct {
	lang.Object
	header http.Header
}

// NewHeaders creates an empty Headers object.
func NewHeaders() *Headers {
	return &Headers{header: make(http.Header)}
}

// Set sets a header key to value.
func (h *Headers) Set(key, value *lang.String) {
	if h != nil && key != nil && value != nil {
		h.header.Set(key.GoString(), value.GoString())
	}
}

// Add appends a header value.
func (h *Headers) Add(key, value *lang.String) {
	if h != nil && key != nil && value != nil {
		h.header.Add(key.GoString(), value.GoString())
	}
}

// Get returns the first value for key.
func (h *Headers) Get(key *lang.String) *lang.String {
	return h.GetFirst(key)
}

// GetFirst returns the first value for key.
func (h *Headers) GetFirst(key *lang.String) *lang.String {
	if h == nil || key == nil {
		return nil
	}
	val := h.header.Get(key.GoString())
	if val == "" {
		return nil
	}
	return lang.NewString(val)
}

// HttpContext models com.sun.net.httpserver.HttpContext.
type HttpContext struct {
	lang.Object
	path    string
	handler any
}

// GetPath returns the context path.
func (c *HttpContext) GetPath() *lang.String {
	if c == nil {
		return lang.NewString("/")
	}
	return lang.NewString(c.path)
}

// GetHandler returns the registered handler.
func (c *HttpContext) GetHandler() any {
	if c == nil {
		return nil
	}
	return c.handler
}

// HttpExchange is the native shim for com.sun.net.httpserver.HttpExchange.
type HttpExchange struct {
	lang.Object
	req            *http.Request
	writer         http.ResponseWriter
	reqHeaders     *Headers
	respHeaders    *Headers
	reqBody        *InputStream
	respBody       *OutputStream
	respStatus     int32
	headersWritten bool
	attributes     map[string]any
}

func newHttpExchange(w http.ResponseWriter, r *http.Request) *HttpExchange {
	reqH := NewHeaders()
	if r != nil {
		reqH.header = r.Header
	}
	return &HttpExchange{
		req:         r,
		writer:      w,
		reqHeaders:  reqH,
		respHeaders: NewHeaders(),
		attributes:  make(map[string]any),
		respStatus:  200,
	}
}

// GetRequestMethod returns the request HTTP method (GET, POST, etc.).
func (e *HttpExchange) GetRequestMethod() *lang.String {
	if e == nil || e.req == nil {
		return lang.NewString("GET")
	}
	return lang.NewString(e.req.Method)
}

// GetRequestURI returns the request URI.
func (e *HttpExchange) GetRequestURI() *URI {
	if e == nil || e.req == nil || e.req.URL == nil {
		return NewURI("/")
	}
	return NewURI(e.req.URL.RequestURI())
}

// GetRequestHeaders returns the request headers.
func (e *HttpExchange) GetRequestHeaders() *Headers {
	if e == nil {
		return NewHeaders()
	}
	return e.reqHeaders
}

// GetResponseHeaders returns the response headers.
func (e *HttpExchange) GetResponseHeaders() *Headers {
	if e == nil {
		return NewHeaders()
	}
	return e.respHeaders
}

// GetRequestBody returns an InputStream for reading the request body.
func (e *HttpExchange) GetRequestBody() *InputStream {
	if e == nil || e.req == nil || e.req.Body == nil {
		return NewInputStream(nil)
	}
	if e.reqBody == nil {
		e.reqBody = NewInputStream(e.req.Body)
	}
	return e.reqBody
}

// GetResponseBody returns an OutputStream for writing the response body.
func (e *HttpExchange) GetResponseBody() *OutputStream {
	if e == nil || e.writer == nil {
		return NewOutputStream(nil)
	}
	if e.respBody == nil {
		e.respBody = NewOutputStream(e.writer)
	}
	return e.respBody
}

// SendResponseHeaders sends response headers and status code.
func (e *HttpExchange) SendResponseHeaders(rCode int32, responseLength int64) {
	if e == nil || e.writer == nil || e.headersWritten {
		return
	}
	e.respStatus = rCode
	e.headersWritten = true

	if e.respHeaders != nil {
		for k, values := range e.respHeaders.header {
			for _, v := range values {
				e.writer.Header().Add(k, v)
			}
		}
	}
	if responseLength > 0 {
		e.writer.Header().Set("Content-Length", strconv.FormatInt(responseLength, 10))
	}
	e.writer.WriteHeader(int(rCode))
}

// GetResponseCode returns the status code sent.
func (e *HttpExchange) GetResponseCode() int32 {
	if e == nil {
		return 0
	}
	return e.respStatus
}

// SetAttribute stores a request attribute.
func (e *HttpExchange) SetAttribute(name *lang.String, value any) {
	if e != nil && name != nil {
		e.attributes[name.GoString()] = value
	}
}

// GetAttribute retrieves a request attribute.
func (e *HttpExchange) GetAttribute(name *lang.String) any {
	if e == nil || name == nil {
		return nil
	}
	return e.attributes[name.GoString()]
}

// Close flushes and completes the exchange.
func (e *HttpExchange) Close() {
	if e == nil {
		return
	}
	if !e.headersWritten {
		e.SendResponseHeaders(e.respStatus, 0)
	}
	if e.respBody != nil {
		e.respBody.Close()
	}
	if e.reqBody != nil {
		e.reqBody.Close()
	}
}

// HttpServer is the native shim for com.sun.net.httpserver.HttpServer,
// powered directly by Go's net/http standard library.
type HttpServer struct {
	lang.Object
	mu       sync.Mutex
	server   *http.Server
	mux      *http.ServeMux
	listener net.Listener
	addr     *InetSocketAddress
	contexts map[string]*HttpContext
	started  bool
	closed   bool
}

// HttpServerCreate implements HttpServer.create(InetSocketAddress, int).
func HttpServerCreate(args ...any) *HttpServer {
	s := &HttpServer{
		mux:      http.NewServeMux(),
		contexts: make(map[string]*HttpContext),
	}
	if len(args) > 0 {
		if addr, ok := args[0].(*InetSocketAddress); ok && addr != nil {
			s.addr = addr
		}
	}
	return s
}

// Init initializes the HttpServer.
func (s *HttpServer) Init(args ...any) *HttpServer {
	if s == nil {
		return s
	}
	s.mux = http.NewServeMux()
	s.contexts = make(map[string]*HttpContext)
	if len(args) > 0 {
		if addr, ok := args[0].(*InetSocketAddress); ok && addr != nil {
			s.addr = addr
		}
	}
	return s
}

// Bind sets the address and backlog.
func (s *HttpServer) Bind(addr *InetSocketAddress, backlog int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addr = addr
}

// CreateContext registers a path and handler on the server.
func (s *HttpServer) CreateContext(path *lang.String, handler any) *HttpContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := "/"
	if path != nil {
		p = path.GoString()
	}
	ctx := &HttpContext{path: p, handler: handler}
	s.contexts[p] = ctx

	s.mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
		exchange := newHttpExchange(w, r)
		defer exchange.Close()

		if h, ok := handler.(HttpHandler); ok {
			h.Handle(exchange)
			return
		}
		if h, ok := handler.(interface{ Handle(*HttpExchange) }); ok {
			h.Handle(exchange)
			return
		}
		if fn, ok := handler.(func(*HttpExchange)); ok {
			fn(exchange)
			return
		}
		http.Error(w, "handler not callable", http.StatusInternalServerError)
	})
	return ctx
}

// Start boots the HTTP server in a background goroutine.
func (s *HttpServer) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	port := int32(0)
	if s.addr != nil {
		port = s.addr.Port
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	s.listener = ln
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		s.addr = &InetSocketAddress{Host: "127.0.0.1", Port: int32(tcp.Port)}
	}
	s.server = &http.Server{Handler: s.mux}
	s.started = true
	go func() {
		_ = s.server.Serve(ln)
	}()
}

// Stop shuts down the HTTP server.
func (s *HttpServer) Stop(delay int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.server != nil {
		_ = s.server.Close()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// GetAddress returns the listening InetSocketAddress.
func (s *HttpServer) GetAddress() *InetSocketAddress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}
