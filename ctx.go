package fuego

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-fuego/fuego/internal"
)

const (
	maxBodySize = 1048576
)

type ContextNoBody = ContextWithBody[any]

// ContextWithBody is the context of the request.
// It contains the request body, the path parameters, the query parameters, and the HTTP request.
// Please do not use a pointer type as parameter.
type ContextWithBody[B any] interface {
	context.Context

	ValidableCtx

	// Body returns the body of the request.
	// If (*B) implements [InTransformer], it will be transformed after deserialization.
	// It caches the result, so it can be called multiple times.
	Body() (B, error)

	// MustBody works like Body, but panics if there is an error.
	MustBody() B

	// PathParam returns the path parameter with the given name.
	// If it does not exist, it returns an empty string.
	// Example:
	//   fuego.Get(s, "/recipes/{recipe_id}", func(c fuego.ContextNoBody) (any, error) {
	//	 	id := c.PathParam("recipe_id")
	//   	...
	//   })
	PathParam(name string) string

	QueryParam(name string) string
	QueryParamArr(name string) []string
	QueryParamInt(name string) int // If the query parameter is not provided or is not an int, it returns the default given value. Use [Ctx.QueryParamIntErr] if you want to know if the query parameter is erroneous.
	QueryParamIntErr(name string) (int, error)
	QueryParamBool(name string) bool // If the query parameter is not provided or is not a bool, it returns the default given value. Use [Ctx.QueryParamBoolErr] if you want to know if the query parameter is erroneous.
	QueryParamBoolErr(name string) (bool, error)
	QueryParams() url.Values

	MainLang() string   // ex: fr. MainLang returns the main language of the request. It is the first language of the Accept-Language header. To get the main locale (ex: fr-CA), use [Ctx.MainLocale].
	MainLocale() string // ex: en-US. MainLocale returns the main locale of the request. It is the first locale of the Accept-Language header. To get the main language (ex: en), use [Ctx.MainLang].

	// Render renders the given templates with the given data.
	// Example:
	//   fuego.Get(s, "/recipes", func(c fuego.ContextNoBody) (any, error) {
	//   	recipes, _ := rs.Queries.GetRecipes(c.Context())
	//   		...
	//   	return c.Render("pages/recipes.page.html", recipes)
	//   })
	// For the Go templates reference, see https://pkg.go.dev/html/template
	//
	// [templateGlobsToOverride] is a list of templates to override.
	// For example, if you have 2 conflicting templates
	//   - with the same name "partials/aaa/nav.partial.html" and "partials/bbb/nav.partial.html"
	//   - or two templates with different names, but that define the same block "page" for example,
	// and you want to override one above the other, you can do:
	//   c.Render("admin.page.html", recipes, "partials/aaa/nav.partial.html")
	// By default, [templateToExecute] is added to the list of templates to override.
	Render(templateToExecute string, data any, templateGlobsToOverride ...string) (CtxRenderer, error)

	Cookie(name string) (*http.Cookie, error) // Get request cookie
	SetCookie(cookie http.Cookie)             // Sets response cookie
	Header(key string) string                 // Get request header
	SetHeader(key, value string)              // Sets response header

	// Returns the underlying net/http, gin or echo context.
	//
	// Usage:
	//  ctx := c.Context() // net/http: the [context.Context] of the *http.Request
	//  ctx := c.Context().(*gin.Context) // gin: Safe because the underlying context is always a [gin.Context]
	//  ctx := c.Context().(echo.Context) // echo: Safe because the underlying context is always a [echo.Context]
	Context() context.Context

	Request() *http.Request        // Request returns the underlying HTTP request.
	Response() http.ResponseWriter // Response returns the underlying HTTP response writer.

	// SetStatus sets the status code of the response.
	// Alias to http.ResponseWriter.WriteHeader.
	SetStatus(code int)

	// Redirect redirects to the given url with the given status code.
	// Example:
	//   fuego.Get(s, "/recipes", func(c fuego.ContextNoBody) (any, error) {
	//   	...
	//   	return c.Redirect(301, "/recipes-list")
	//   })
	Redirect(code int, url string) (any, error)
}

// NewNetHTTPContext returns a new context. It is used internally by Fuego. You probably want to use Ctx[B] instead.
func NewNetHTTPContext[B any](route BaseRoute, w http.ResponseWriter, r *http.Request, options readOptions) *netHttpContext[B] {
	c := &netHttpContext[B]{
		CommonContext: internal.CommonContext[B]{
			CommonCtx:         r.Context(),
			UrlValues:         r.URL.Query(),
			OpenAPIParams:     route.Params,
			DefaultStatusCode: route.DefaultStatusCode,
		},
		Req:         r,
		Res:         w,
		readOptions: options,
	}

	return c
}

// ContextWithBody is the same as fuego.ContextNoBody, but
// has a Body. The Body type parameter represents the expected data type
// from http.Request.Body. Please do not use a pointer as a type parameter.
type netHttpContext[Body any] struct {
	Res http.ResponseWriter

	fs fs.FS

	body *Body // Cache the body in request context, because it is not possible to read an HTTP request body multiple times.

	Req       *http.Request
	templates *template.Template

	serializer      Sender
	errorSerializer ErrorSender

	internal.CommonContext[Body]

	readOptions readOptions
}

var (
	_ ContextWithBody[any]    = &netHttpContext[any]{}    // Check that ContextWithBody implements Ctx.
	_ ContextWithBody[string] = &netHttpContext[string]{} // Check that ContextWithBody implements Ctx.
	_ ValidableCtx            = &netHttpContext[any]{}    // Check that ContextWithBody implements ValidableCtx.
)

// SetStatus sets the status code of the response.
// Alias to http.ResponseWriter.WriteHeader.
func (c netHttpContext[B]) SetStatus(code int) {
	c.Res.WriteHeader(code)
}

// readOptions are options for reading the request body.
type readOptions struct {
	MaxBodySize           int64
	DisallowUnknownFields bool
	LogBody               bool
}

func (c netHttpContext[B]) Redirect(code int, url string) (any, error) {
	http.Redirect(c.Res, c.Req, url, code)

	return nil, nil
}

// Get request header
func (c netHttpContext[B]) Header(key string) string {
	return c.Request().Header.Get(key)
}

// Has request header
func (c netHttpContext[B]) HasHeader(key string) bool {
	return c.Header(key) != ""
}

// Sets response header
func (c netHttpContext[B]) SetHeader(key, value string) {
	c.Response().Header().Set(key, value)
}

// Get request cookie
func (c netHttpContext[B]) Cookie(name string) (*http.Cookie, error) {
	return c.Request().Cookie(name)
}

// Has request cookie
func (c netHttpContext[B]) HasCookie(name string) bool {
	_, err := c.Cookie(name)
	return err == nil
}

// Sets response cookie
func (c netHttpContext[B]) SetCookie(cookie http.Cookie) {
	http.SetCookie(c.Response(), &cookie)
}

// Render renders the given templates with the given data.
// It returns just an empty string, because the response is written directly to the http.ResponseWriter.
//
// Init templates if not already done.
// This has the side effect of making the Render method static, meaning
// that the templates will be parsed only once, removing
// the need to parse the templates on each request but also preventing
// to dynamically use new templates.
func (c netHttpContext[B]) Render(templateToExecute string, data any, layoutsGlobs ...string) (CtxRenderer, error) {
	return &StdRenderer{
		templateToExecute: templateToExecute,
		templates:         c.templates,
		layoutsGlobs:      layoutsGlobs,
		fs:                c.fs,
		data:              data,
	}, nil
}

// PathParams returns the path parameters of the request.
func (c netHttpContext[B]) PathParam(name string) string {
	return c.Req.PathValue(name)
}

func (c netHttpContext[B]) MainLang() string {
	return strings.Split(c.MainLocale(), "-")[0]
}

func (c netHttpContext[B]) MainLocale() string {
	return strings.Split(c.Req.Header.Get("Accept-Language"), ",")[0]
}

// Request returns the HTTP request.
func (c netHttpContext[B]) Request() *http.Request {
	return c.Req
}

// Response returns the HTTP response writer.
func (c netHttpContext[B]) Response() http.ResponseWriter {
	return c.Res
}

// MustBody works like Body, but panics if there is an error.
func (c *netHttpContext[B]) MustBody() B {
	b, err := c.Body()
	if err != nil {
		panic(err)
	}
	return b
}

// Body returns the body of the request.
// If (*B) implements [InTransformer], it will be transformed after deserialization.
// It caches the result, so it can be called multiple times.
// The reason the body is cached is that it is impossible to read an HTTP request body multiple times, not because of performance.
// For decoding, it uses the Content-Type header. If it is not set, defaults to application/json.
func (c *netHttpContext[B]) Body() (B, error) {
	if c.body != nil {
		return *c.body, nil
	}

	body, err := body[B](*c)
	c.body = &body
	return body, err
}

// Serialize serializes the given data to the response. It uses the Content-Type header to determine the serialization format.
func (c netHttpContext[B]) Serialize(data any) error {
	if c.serializer == nil {
		return Send(c.Res, c.Req, data)
	}
	return c.serializer(c.Res, c.Req, data)
}

// SerializeError serializes the given error to the response. It uses the Content-Type header to determine the serialization format.
func (c netHttpContext[B]) SerializeError(err error) {
	if c.errorSerializer == nil {
		SendError(c.Res, c.Req, err)
		return
	}
	c.errorSerializer(c.Res, c.Req, err)
}

// setDefaultStatusCode sets the default status code of the response.
func (c netHttpContext[B]) SetDefaultStatusCode() {
	if c.DefaultStatusCode != 0 {
		c.SetStatus(c.DefaultStatusCode)
	}
}

func body[B any](c netHttpContext[B]) (B, error) {
	// Limit the size of the request body.
	if c.readOptions.MaxBodySize != 0 {
		c.Req.Body = http.MaxBytesReader(nil, c.Req.Body, c.readOptions.MaxBodySize)
	}

	timeDeserialize := time.Now()

	var body B
	var err error
	switch c.Req.Header.Get("Content-Type") {
	case "text/plain":
		s, errReadingString := readString[string](c.Req.Context(), c.Req.Body, c.readOptions)
		body = any(s).(B)
		err = errReadingString
	case "application/x-www-form-urlencoded", "multipart/form-data":
		body, err = readURLEncoded[B](c.Req, c.readOptions)
	case "application/xml":
		body, err = readXML[B](c.Req.Context(), c.Req.Body, c.readOptions)
	case "application/x-yaml", "text/yaml; charset=utf-8", "application/yaml": // https://www.rfc-editor.org/rfc/rfc9512.html
		body, err = readYAML[B](c.Req.Context(), c.Req.Body, c.readOptions)
	case "application/octet-stream":
		// Read c.Req Body to bytes
		bytes, err := io.ReadAll(c.Req.Body)
		if err != nil {
			return body, err
		}
		respBytes, ok := any(bytes).(B)
		if !ok {
			return body, fmt.Errorf("could not convert bytes to %T. To read binary data from the request, use []byte as the body type", body)
		}
		body = respBytes
	case "application/json":
		fallthrough
	default:
		body, err = readJSON[B](c.Req.Context(), c.Req.Body, c.readOptions)
	}

	c.Res.Header().Add("Server-Timing", Timing{"deserialize", "controller > deserialize", time.Since(timeDeserialize)}.String())

	return body, err
}
