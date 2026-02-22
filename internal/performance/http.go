// Package performance provides high-performance HTTP utilities
package performance

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

// OptimizedHTTPClient creates a highly optimized HTTP client
// with tuned connection pooling and reduced latency
func OptimizedHTTPClient() *http.Client {
	return &http.Client{
		Transport: OptimizedTransport(),
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// OptimizedTransport creates an optimized HTTP transport
func OptimizedTransport() *http.Transport {
	return &http.Transport{
		// Connection pooling
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,

		// TLS settings for speed
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},

		// Connection settings
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,

		// Timeouts
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,

		// Disable compression (handle manually if needed)
		DisableCompression: false,

		// HTTP/2 support
		ForceAttemptHTTP2: true,
	}
}

// FastHTTPClient is a lightweight HTTP client for simple requests
type FastHTTPClient struct {
	client    *http.Client
	transport *http.Transport
	mu        sync.RWMutex
}

// NewFastHTTPClient creates a new fast HTTP client
func NewFastHTTPClient() *FastHTTPClient {
	transport := OptimizedTransport()
	return &FastHTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		transport: transport,
	}
}

// Do executes an HTTP request
func (c *FastHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Get performs a GET request
func (c *FastHTTPClient) Get(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.client.Do(req)
}

// Head performs a HEAD request
func (c *FastHTTPClient) Head(url string) (*http.Response, error) {
	return c.client.Head(url)
}

// CloseIdleConnections closes idle connections
func (c *FastHTTPClient) CloseIdleConnections() {
	c.transport.CloseIdleConnections()
}

// SetTimeout sets the client timeout
func (c *FastHTTPClient) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client.Timeout = timeout
}

// HTTPPool is a pool of reusable HTTP clients
type HTTPPool struct {
	pool sync.Pool
}

// NewHTTPPool creates a new HTTP client pool
func NewHTTPPool() *HTTPPool {
	return &HTTPPool{
		pool: sync.Pool{
			New: func() interface{} {
				return NewFastHTTPClient()
			},
		},
	}
}

// Get retrieves a client from the pool
func (p *HTTPPool) Get() *FastHTTPClient {
	return p.pool.Get().(*FastHTTPClient)
}

// Put returns a client to the pool
func (p *HTTPPool) Put(client *FastHTTPClient) {
	p.pool.Put(client)
}

// ResponseBufferPool is a pool for response buffers
type ResponseBufferPool struct {
	pool sync.Pool
}

// NewResponseBufferPool creates a new response buffer pool
func NewResponseBufferPool(size int) *ResponseBufferPool {
	return &ResponseBufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *ResponseBufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool
func (p *ResponseBufferPool) Put(buf []byte) {
	p.pool.Put(buf)
}

// FastReadAll reads response body efficiently
func FastReadAll(r io.Reader) ([]byte, error) {
	// Try to get size from limited readers
	size := 4096 // Default buffer size

	buf := make([]byte, 0, size)
	tmp := make([]byte, 4096)

	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
	}
}

// FastReadBody reads HTTP response body with size limit
func FastReadBody(resp *http.Response, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}

	defer resp.Body.Close()

	// Check Content-Length
	if resp.ContentLength > maxSize {
		return nil, fmt.Errorf("response body too large: %d > %d", resp.ContentLength, maxSize)
	}

	// Pre-allocate if size is known
	var buf []byte
	if resp.ContentLength > 0 {
		buf = make([]byte, 0, resp.ContentLength)
	} else {
		buf = make([]byte, 0, 4096)
	}

	tmp := make([]byte, 4096)
	var total int64

	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > maxSize {
				return nil, fmt.Errorf("response body exceeds max size")
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
	}
}

// HTTPRequestBuilder helps build HTTP requests efficiently
type HTTPRequestBuilder struct {
	method  string
	url     string
	headers map[string]string
	body    []byte
	ctx     context.Context
}

// NewHTTPRequestBuilder creates a new request builder
func NewHTTPRequestBuilder(method, url string) *HTTPRequestBuilder {
	return &HTTPRequestBuilder{
		method:  method,
		url:     url,
		headers: make(map[string]string),
		ctx:     context.Background(),
	}
}

// WithContext sets the context
func (b *HTTPRequestBuilder) WithContext(ctx context.Context) *HTTPRequestBuilder {
	b.ctx = ctx
	return b
}

// WithHeader adds a header
func (b *HTTPRequestBuilder) WithHeader(key, value string) *HTTPRequestBuilder {
	b.headers[key] = value
	return b
}

// WithHeaders adds multiple headers
func (b *HTTPRequestBuilder) WithHeaders(headers map[string]string) *HTTPRequestBuilder {
	for k, v := range headers {
		b.headers[k] = v
	}
	return b
}

// WithBody sets the request body
func (b *HTTPRequestBuilder) WithBody(body []byte) *HTTPRequestBuilder {
	b.body = body
	return b
}

// Build creates the HTTP request
func (b *HTTPRequestBuilder) Build() (*http.Request, error) {
	var body io.Reader
	if len(b.body) > 0 {
		body = newBytesReader(b.body)
	}

	req, err := http.NewRequestWithContext(b.ctx, b.method, b.url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// bytesReader is a simple bytes reader for request body
type bytesReader struct {
	s []byte
	i int
}

// newBytesReader creates a new bytes reader
func newBytesReader(b []byte) *bytesReader {
	return &bytesReader{
		s: b,
		i: 0,
	}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func (r *bytesReader) Close() error {
	return nil
}

// HTTPTiming provides detailed HTTP timing metrics
type HTTPTiming struct {
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration
	Total            time.Duration
}

// WithTiming performs an HTTP request with detailed timing
func WithTiming(client *http.Client, req *http.Request) (*http.Response, *HTTPTiming, error) {
	var timing HTTPTiming
	var start, tlsHandshake, wroteRequest, gotFirstByte time.Time

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			start = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			timing.DNSLookup = time.Since(start)
		},
		ConnectStart: func(network, addr string) {
			if timing.DNSLookup == 0 {
				// Using cached DNS
				start = time.Now()
			}
		},
		ConnectDone: func(network, addr string, err error) {
			timing.TCPConnection = time.Since(start)
		},
		TLSHandshakeStart: func() {
			tlsHandshake = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			timing.TLSHandshake = time.Since(tlsHandshake)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			wroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
			timing.ServerProcessing = time.Since(wroteRequest)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	// Read body for content transfer timing
	body, err := FastReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	timing.ContentTransfer = time.Since(gotFirstByte)
	timing.Total = time.Since(start)

	// Restore body for further processing
	resp.Body = io.NopCloser(newBytesReader(body))

	return resp, &timing, nil
}

// ConnectionPool manages a pool of pre-warmed connections
type ConnectionPool struct {
	network string
	addrs   []string
	conns   chan net.Conn
	mu      sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(network string, addrs []string, size int) *ConnectionPool {
	return &ConnectionPool{
		network: network,
		addrs:   addrs,
		conns:   make(chan net.Conn, size),
	}
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get() (net.Conn, error) {
	select {
	case conn := <-p.conns:
		return conn, nil
	default:
		// Create new connection
		if len(p.addrs) == 0 {
			return nil, fmt.Errorf("no addresses available")
		}
		dialer := &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return dialer.Dial(p.network, p.addrs[0])
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}
	select {
	case p.conns <- conn:
	default:
		// Pool is full, close connection
		conn.Close()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	close(p.conns)
	for conn := range p.conns {
		conn.Close()
	}
}
