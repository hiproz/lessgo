// +build !appengine

package fasthttp

import (
	"sync"

	"github.com/lessgo/lessgo"
	"github.com/lessgo/lessgo/engine"
	"github.com/lessgo/lessgo/logs"
	"github.com/valyala/fasthttp"
)

type (
	// Server implements `engine.Server`.
	Server struct {
		*fasthttp.Server
		config  engine.Config
		handler engine.Handler
		logger  logs.Logger
		pool    *pool
	}

	pool struct {
		request        sync.Pool
		response       sync.Pool
		requestHeader  sync.Pool
		responseHeader sync.Pool
		url            sync.Pool
	}
)

// New returns `fasthttp.Server` with provided listen address.
func New(addr string) engine.Server {
	c := engine.Config{Address: addr}
	return WithConfig(c)
}

// WithTLS returns `Server` instance with provided TLS config.
func WithTLS(addr, certfile, keyfile string) engine.Server {
	c := engine.Config{
		Address:     addr,
		TLSCertfile: certfile,
		TLSKeyfile:  keyfile,
	}
	return WithConfig(c)
}

// WithConfig returns `Server` instance with provided config.
func WithConfig(c engine.Config) engine.Server {
	var s *Server
	s = &Server{
		Server: new(fasthttp.Server),
		config: c,
		pool: &pool{
			request: sync.Pool{
				New: func() interface{} {
					return &Request{}
				},
			},
			response: sync.Pool{
				New: func() interface{} {
					return &Response{logger: s.logger}
				},
			},
			requestHeader: sync.Pool{
				New: func() interface{} {
					return &RequestHeader{}
				},
			},
			responseHeader: sync.Pool{
				New: func() interface{} {
					return &ResponseHeader{}
				},
			},
			url: sync.Pool{
				New: func() interface{} {
					return &URL{}
				},
			},
		},
		handler: engine.HandlerFunc(func(rq engine.Request, rs engine.Response) {
			s.logger.Error("handler not set, use `SetHandler()` to set it.")
		}),
		logger: logs.GlobalLogger,
	}
	s.Handler = s.ServeHTTP
	return s
}

// SetHandler implements `engine.Server#SetHandler` function.
func (s *Server) SetHandler(h engine.Handler) {
	s.handler = h
}

// SetLogger implements `engine.Server#SetLogger` function.
func (s *Server) SetLogger(l logs.Logger) {
	s.logger = l
}

// Start implements `engine.Server#Start` function.
func (s *Server) Start() error {
	if s.config.Listener == nil {
		return s.startDefaultListener()
	}
	return s.startCustomListener()

}

func (s *Server) startDefaultListener() error {
	c := s.config
	if c.TLSCertfile != "" && c.TLSKeyfile != "" {
		return s.ListenAndServeTLS(c.Address, c.TLSCertfile, c.TLSKeyfile)
	}
	return s.ListenAndServe(c.Address)
}

func (s *Server) startCustomListener() error {
	c := s.config
	if c.TLSCertfile != "" && c.TLSKeyfile != "" {
		return s.ServeTLS(c.Listener, c.TLSCertfile, c.TLSKeyfile)
	}
	return s.Serve(c.Listener)
}

func (s *Server) ServeHTTP(c *fasthttp.RequestCtx) {
	// Request
	rq := s.pool.request.Get().(*Request)
	rqHdr := s.pool.requestHeader.Get().(*RequestHeader)
	rqURL := s.pool.url.Get().(*URL)
	rqHdr.reset(&c.Request.Header)
	rqURL.reset(c.URI())
	rq.reset(c, rqHdr, rqURL)

	// Response
	rs := s.pool.response.Get().(*Response)
	rsHdr := s.pool.responseHeader.Get().(*ResponseHeader)
	rsHdr.reset(&c.Response.Header)
	rs.reset(c, rsHdr)

	s.handler.ServeHTTP(rq, rs)

	// Return to pool
	s.pool.request.Put(rq)
	s.pool.requestHeader.Put(rqHdr)
	s.pool.url.Put(rqURL)
	s.pool.response.Put(rs)
	s.pool.responseHeader.Put(rsHdr)
}

// WrapHandler wraps `fasthttp.RequestHandler` into `lessgo.HandlerFunc`.
func WrapHandler(h fasthttp.RequestHandler) lessgo.HandlerFunc {
	return func(c lessgo.Context) error {
		rq := c.Request().(*Request)
		rs := c.Response().(*Response)
		ctx := rq.RequestCtx
		h(ctx)
		rs.status = ctx.Response.StatusCode()
		rs.size = int64(ctx.Response.Header.ContentLength())
		return nil
	}
}

// WrapMiddleware wraps `fasthttp.RequestHandler` into `lessgo.MiddlewareFunc`
func WrapMiddleware(h fasthttp.RequestHandler) lessgo.MiddlewareFunc {
	return func(next lessgo.HandlerFunc) lessgo.HandlerFunc {
		return func(c lessgo.Context) error {
			rq := c.Request().(*Request)
			rs := c.Response().(*Response)
			ctx := rq.RequestCtx
			h(ctx)
			rs.status = ctx.Response.StatusCode()
			rs.size = int64(ctx.Response.Header.ContentLength())
			return next(c)
		}
	}
}
