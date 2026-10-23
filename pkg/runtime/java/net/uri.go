package jnet

import (
	"bytes"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

// URI is the native shim for java.net.URI.
type URI struct {
	lang.Object
	raw    string
	parsed *url.URL
}

// NewURI constructs a URI from a Go string.
func NewURI(uriStr string) *URI {
	u, _ := url.Parse(uriStr)
	return &URI{raw: uriStr, parsed: u}
}

// URICreate implements static URI.create(String).
func URICreate(s *lang.String) *URI {
	if s == nil {
		return NewURI("")
	}
	return NewURI(s.GoString())
}

// Init initializes the URI.
func (u *URI) Init(args ...any) *URI {
	if u == nil || len(args) == 0 {
		return u
	}
	str := ""
	if s, ok := args[0].(*lang.String); ok && s != nil {
		str = s.GoString()
	} else if gs, ok := args[0].(string); ok {
		str = gs
	}
	u.raw = str
	u.parsed, _ = url.Parse(str)
	return u
}

// GetPath returns the path component.
func (u *URI) GetPath() *lang.String {
	if u == nil || u.parsed == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.parsed.Path)
}

// GetQuery returns the query component.
func (u *URI) GetQuery() *lang.String {
	if u == nil || u.parsed == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.parsed.RawQuery)
}

// GetHost returns the host component.
func (u *URI) GetHost() *lang.String {
	if u == nil || u.parsed == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.parsed.Hostname())
}

// GetPort returns the port component, or -1 if not specified.
func (u *URI) GetPort() int32 {
	if u == nil || u.parsed == nil {
		return -1
	}
	p := u.parsed.Port()
	if p == "" {
		return -1
	}
	val, err := strconv.Atoi(p)
	if err != nil {
		return -1
	}
	return int32(val)
}

// ToString renders the URI.
func (u *URI) ToString() *lang.String {
	if u == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.raw)
}

// URL is the native shim for java.net.URL.
type URL struct {
	lang.Object
	raw    string
	parsed *url.URL
}

// Init initializes the URL.
func (u *URL) Init(args ...any) *URL {
	if u == nil || len(args) == 0 {
		return u
	}
	str := ""
	if s, ok := args[0].(*lang.String); ok && s != nil {
		str = s.GoString()
	} else if gs, ok := args[0].(string); ok {
		str = gs
	}
	u.raw = str
	u.parsed, _ = url.Parse(str)
	return u
}

// OpenConnection opens an HTTP connection.
func (u *URL) OpenConnection() *HttpURLConnection {
	if u == nil || u.parsed == nil {
		panic(lang.NewIOException(lang.NewString("invalid URL")))
	}
	return &HttpURLConnection{
		url:     u.parsed,
		method:  "GET",
		headers: make(map[string]string),
	}
}

// OpenStream opens an input stream directly to the URL.
func (u *URL) OpenStream() *InputStream {
	conn := u.OpenConnection()
	return conn.GetInputStream()
}

// GetPath returns the URL path.
func (u *URL) GetPath() *lang.String {
	if u == nil || u.parsed == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.parsed.Path)
}

// ToString renders the URL string.
func (u *URL) ToString() *lang.String {
	if u == nil {
		return lang.NewString("")
	}
	return lang.NewString(u.raw)
}

// HttpURLConnection is the native shim for java.net.HttpURLConnection.
type HttpURLConnection struct {
	lang.Object
	url     *url.URL
	method  string
	headers map[string]string
	resp    *http.Response
	reqBody bytes.Buffer
}

// SetRequestMethod sets the HTTP request method.
func (c *HttpURLConnection) SetRequestMethod(method *lang.String) {
	if c != nil && method != nil {
		c.method = strings.ToUpper(method.GoString())
	}
}

// SetRequestProperty sets a request header.
func (c *HttpURLConnection) SetRequestProperty(key, value *lang.String) {
	if c != nil && key != nil && value != nil {
		c.headers[key.GoString()] = value.GoString()
	}
}

func (c *HttpURLConnection) execute() {
	if c.resp != nil {
		return
	}
	req, err := http.NewRequest(c.method, c.url.String(), &c.reqBody)
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(lang.NewIOException(lang.NewString(err.Error())))
	}
	c.resp = resp
}

// GetResponseCode triggers the request and returns the status code.
func (c *HttpURLConnection) GetResponseCode() int32 {
	if c == nil {
		return 0
	}
	c.execute()
	return int32(c.resp.StatusCode)
}

// GetInputStream returns the response body stream.
func (c *HttpURLConnection) GetInputStream() *InputStream {
	if c == nil {
		return NewInputStream(nil)
	}
	c.execute()
	return NewInputStream(c.resp.Body)
}

// GetOutputStream returns a stream for writing the request body.
func (c *HttpURLConnection) GetOutputStream() *OutputStream {
	if c == nil {
		return NewOutputStream(nil)
	}
	return NewOutputStream(&c.reqBody)
}

// Disconnect cleans up the connection.
func (c *HttpURLConnection) Disconnect() {
	if c != nil && c.resp != nil && c.resp.Body != nil {
		_ = c.resp.Body.Close()
	}
}
