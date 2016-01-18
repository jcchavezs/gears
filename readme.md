# gears
--
    import "github.com/zgiber/gears"


## Usage

```go
var BGContext context.Context
```
BGContext is the background context for all gears middleware

#### func  NewError

```go
func NewError(c context.Context, code int, message string) context.Context
```
NewError sets the error on the context and returns the canceled context.

#### type Gear

```go
type Gear func(c context.Context, w http.ResponseWriter, r *http.Request) context.Context
```

Gear is a context aware middleware function signature

#### func  Chain

```go
func Chain(gears ...Gear) Gear
```
Chain multiple middleware

#### type HTTPError

```go
type HTTPError struct {
}
```

HTTPError contains code and message and implements error interface

#### func  NewHTTPError

```go
func NewHTTPError(code int, message string) *HTTPError
```
NewHTTPError returns a HTTPError as an error interface

#### func (*HTTPError) Error

```go
func (err *HTTPError) Error() string
```

#### type Handler

```go
type Handler struct {
}
```

Handler is a context aware http request handler

#### func  NewHandler

```go
func NewHandler(fn func(c context.Context, w http.ResponseWriter, r *http.Request), gears ...Gear) *Handler
```
NewHandler returns a pointer to a Handler struct which implements http.Handler
interface. This is a convenient way to construct context aware gear.Handlers
which can be used with standard http routers.

#### func (*Handler) ServeHTTP

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```
