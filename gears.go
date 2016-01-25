package gears

import (
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/net/context"
)

// BGContext is the background context for all
// gears middleware
var BGContext context.Context

func init() {
	BGContext = context.Background()
}

// ContextHandler is a function signature for handers which require context
type ContextHandler func(c context.Context, w http.ResponseWriter, r *http.Request)

// Handler is a context aware http request handler
type handler struct {
	//fn   func(c context.Context, w http.ResponseWriter, r *http.Request)
	gear Gear
}

// Gear is a context aware middleware function signature
type Gear func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context

// New Gear is constructed by taking either of the following types as input;
// func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context
// func(c context.Context, w http.ResponseWriter, r *http.Request)
// http.Handler
// Passing other types will return error.
func New(fn interface{}) Gear {
	switch t := fn.(type) {
	case func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context:
		return Gear(t)
	case func(c context.Context, w http.ResponseWriter, r *http.Request):
		return wrapContextHandler(t)
	case http.Handler:
		return wrapHandler(t)
	default:
		panic("invalid type")
	}
}

func wrapHandler(h http.Handler) Gear {
	return func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
		h.ServeHTTP(w, r)
		return c
	}
}

func wrapContextHandler(h ContextHandler) Gear {
	return func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
		h(c, w, r)
		return c
	}
}

// httpError contains status code and message
// and implements error interface
type httpError struct {
	status  int
	message string
}

func (err *httpError) Error() string {
	return fmt.Sprintf("%v %s", err.Status(), err.message)
}

func (err *httpError) Status() int {
	return err.status
}

func (err *httpError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{"error": err.Status(), "description": err.Error()})
}

// NewHTTPError returns a httpError as an error interface
func newHTTPError(status int, message string) *httpError {
	return &httpError{status, message}
}

// loggedWriter
type loggedWriter struct {
	status int
	w      http.ResponseWriter
}

func (lw *loggedWriter) WriteHeader(status int) {
	lw.status = status
	lw.w.WriteHeader(status)
}

func (lw *loggedWriter) Header() http.Header {
	return lw.w.Header()
}

func (lw *loggedWriter) Write(b []byte) (int, error) {
	return lw.w.Write(b)
}

// NewHandler returns a http.Handler as a convenient way to construct context aware
// gear.Handlers which can be used with standard http routers.
// fn must have a signature of either func(w http.ResponseWriter, r *http.Request)
// or func(c context.Context, w http.ResponseWriter, r *http.Request)
func NewHandler(gears ...Gear) http.Handler {
	gear := Chain(gears...)
	return &handler{gear}
}

// allows for using simple handlers (those without context in NewHandler)
func withContext(fn func(w http.ResponseWriter, r *http.Request)) ContextHandler {
	return func(c context.Context, w http.ResponseWriter, r *http.Request) {
		fn(w, r)
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, cancel := context.WithCancel(BGContext)
	defer cancel()

	// logged writer
	lw := &loggedWriter{0, w}
	c = h.gear(c, lw, r)
	switch c.Err() {
	case context.Canceled, context.DeadlineExceeded:
		handleError(c, w)
		return
	}
}

// Chain multiple middleware
func Chain(gears ...Gear) Gear {
	return func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
		var localCtx context.Context
		for _, gear := range gears {
			localCtx = gear(c, w, r)
			if localCtx == nil {
				return NewErrorContext(c, NewStatusError(500, "Middleware returned nil context"))
			}
			if localCtx.Err() != nil {
				return localCtx
			}

			c = localCtx
		}

		return c
	}
}

func handleError(c context.Context, w http.ResponseWriter) {

	// handle http error
	errValue := c.Value("error")
	if errValue == nil {
		// error not found, means that a middleware canceled the context
		// signaling that the request is completed, don't do further processing
		return
	}

	statusErr, ok := errValue.(StatusError)
	if !ok {
		// error doesn't implement StatusError
		statusErr = NewStatusError(500, fmt.Sprint(errValue))
	}

	responseBody, err := json.Marshal(statusErr)
	if err != nil {
		// error can't be marshaled
		statusErr = NewStatusError(500, fmt.Sprint(errValue))
	}

	// Write the response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusErr.Status())
	fmt.Fprintln(w, string(responseBody))
}

// NewStatusError sets the error on the context and returns the canceled context.
func NewStatusError(status int, message string) StatusError {
	return &httpError{status, message}
}

// NewErrorContext expects an err which implements StatusError interface, and returns
// a context which has a json formatted error on it.
func NewErrorContext(c context.Context, err StatusError) context.Context {

	var cancel context.CancelFunc
	c, cancel = context.WithCancel(c)
	defer cancel()

	if jsonErr, ok := err.(JSONError); ok {
		return context.WithValue(c, "error", jsonErr)
	}

	return context.WithValue(c, "error", &httpError{err.Status(), err.Error()})
}

// NewCanceledContext return a context which is canceled. It is used for signaling
// to any subsequent handler / gear / middleware in the chain to stop processing the request.
func NewCanceledContext(c context.Context) context.Context {

	var cancel context.CancelFunc
	c, cancel = context.WithCancel(c)
	defer cancel()
	return c

}
