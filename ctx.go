package fuego

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxBodySize = 1048576
)

type contextImpl[B, P any] struct {
	contextWithBodyImpl[B]
	params P
}

func (c contextImpl[B, P]) Params() P {
	return c.params
}

var _ Context[any, any] = &contextImpl[any, any]{}

type (
	ContextNoBody          = Context[any, any]
	ContextWithBody[B any] = Context[B, any]
)

// ctx is the context of the request.
// It contains the request body, the path parameters, the query parameters, and the HTTP request.
// Please do not use a pointer type as parameter.
type Context[B, P any] interface {
	context.Context
	Params() P
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

// NewContextWithBody returns a new context. It is used internally by Fuego. You probably want to use Ctx[B] instead.
func NewContextWithBody[B, P any](w http.ResponseWriter, r *http.Request, options readOptions) Context[B, P] {
	c := &contextImpl[B, P]{
		contextWithBodyImpl: contextWithBodyImpl[B]{
			contextNoBodyImpl: NewContextNoBody(w, r, options),
		},
	}

	return c
}

func NewContextNoBody(w http.ResponseWriter, r *http.Request, options readOptions) contextNoBodyImpl {
	c := contextNoBodyImpl{
		Res: w,
		Req: r,
		readOptions: readOptions{
			DisallowUnknownFields: options.DisallowUnknownFields,
			MaxBodySize:           options.MaxBodySize,
		},
		urlValues: r.URL.Query(),
	}
	return c
}

// ContextWithBody is the same as fuego.ContextNoBody, but
// has a Body. The Body type parameter represents the expected data type
// from http.Request.Body. Please do not use a pointer as a type parameter.
type contextWithBodyImpl[Body any] struct {
	body *Body // Cache the body in request context, because it is not possible to read an HTTP request body multiple times.
	contextNoBodyImpl
}

var (
	_ ContextWithBody[any]    = &contextImpl[any, any]{}    // Check that ContextWithBody implements Ctx.
	_ ContextWithBody[string] = &contextImpl[string, any]{} // Check that ContextWithBody implements Ctx.
)

// ContextNoBody is used when the controller does not have a body.
// It is used as a base context for other Context types.
type contextNoBodyImpl struct {
	Req *http.Request
	Res http.ResponseWriter

	fs        fs.FS
	templates *template.Template

	params    map[string]OpenAPIParam // list of expected query parameters (declared in the OpenAPI spec)
	urlValues url.Values

	readOptions readOptions
}

var _ context.Context = contextNoBodyImpl{} // Check that ContextNoBody implements context.Context.

func (c contextNoBodyImpl) Body() (any, error) {
	slog.Warn("this method should not be called. It probably happened because you passed the context to another controller.")
	return body[map[string]any](c)
}

func (c contextNoBodyImpl) MustBody() any {
	b, err := c.Body()
	if err != nil {
		panic(err)
	}
	return b
}

// SetStatus sets the status code of the response.
// Alias to http.ResponseWriter.WriteHeader.
func (c contextNoBodyImpl) SetStatus(code int) {
	c.Res.WriteHeader(code)
}

// readOptions are options for reading the request body.
type readOptions struct {
	DisallowUnknownFields bool
	MaxBodySize           int64
	LogBody               bool
}

func (c contextNoBodyImpl) Redirect(code int, url string) (any, error) {
	http.Redirect(c.Res, c.Req, url, code)

	return nil, nil
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c contextNoBodyImpl) Deadline() (deadline time.Time, ok bool) {
	return c.Req.Context().Deadline()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c contextNoBodyImpl) Done() <-chan struct{} {
	return c.Req.Context().Done()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c contextNoBodyImpl) Err() error {
	return c.Req.Context().Err()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c contextNoBodyImpl) Value(key any) any {
	return c.Req.Context().Value(key)
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c contextNoBodyImpl) Context() context.Context {
	return c.Req.Context()
}

// Get request header
func (c contextNoBodyImpl) Header(key string) string {
	return c.Request().Header.Get(key)
}

// Has request header
func (c contextNoBodyImpl) HasHeader(key string) bool {
	return c.Header(key) != ""
}

// Sets response header
func (c contextNoBodyImpl) SetHeader(key, value string) {
	c.Response().Header().Set(key, value)
}

// Get request cookie
func (c contextNoBodyImpl) Cookie(name string) (*http.Cookie, error) {
	return c.Request().Cookie(name)
}

// Has request cookie
func (c contextNoBodyImpl) HasCookie(name string) bool {
	_, err := c.Cookie(name)
	return err == nil
}

// Sets response cookie
func (c contextNoBodyImpl) SetCookie(cookie http.Cookie) {
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
func (c contextNoBodyImpl) Render(templateToExecute string, data any, layoutsGlobs ...string) (CtxRenderer, error) {
	return &StdRenderer{
		templateToExecute: templateToExecute,
		templates:         c.templates,
		layoutsGlobs:      layoutsGlobs,
		fs:                c.fs,
		data:              data,
	}, nil
}

// PathParams returns the path parameters of the request.
func (c contextNoBodyImpl) PathParam(name string) string {
	return c.Req.PathValue(name)
}

type QueryParamNotFoundError struct {
	ParamName string
}

func (e QueryParamNotFoundError) Error() string {
	return fmt.Errorf("param %s not found", e.ParamName).Error()
}

type QueryParamInvalidTypeError struct {
	ParamName    string
	ParamValue   string
	ExpectedType string
	Err          error
}

func (e QueryParamInvalidTypeError) Error() string {
	return fmt.Errorf("param %s=%s is not of type %s: %w", e.ParamName, e.ParamValue, e.ExpectedType, e.Err).Error()
}

// QueryParams returns the query parameters of the request. It is a shortcut for c.Req.URL.Query().
func (c contextNoBodyImpl) QueryParams() url.Values {
	return c.urlValues
}

// QueryParamsArr returns an slice of string from the given query parameter.
func (c contextNoBodyImpl) QueryParamArr(name string) []string {
	_, ok := c.params[name]
	if !ok {
		slog.Warn("query parameter not expected in OpenAPI spec", "param", name)
	}
	return c.urlValues[name]
}

// QueryParam returns the query parameter with the given name.
// If it does not exist, it returns an empty string, unless there is a default value declared in the OpenAPI spec.
//
// Example:
//
//	fuego.Get(s, "/test", myController,
//	  option.Query("name", "Name", param.Default("hey"))
//	)
func (c contextNoBodyImpl) QueryParam(name string) string {
	_, ok := c.params[name]
	if !ok {
		slog.Warn("query parameter not expected in OpenAPI spec", "param", name, "expected_one_of", c.params)
	}

	if !c.urlValues.Has(name) {
		defaultValue, _ := c.params[name].Default.(string)
		return defaultValue
	}
	return c.urlValues.Get(name)
}

func (c contextNoBodyImpl) QueryParamIntErr(name string) (int, error) {
	param := c.QueryParam(name)
	if param == "" {
		defaultValue, ok := c.params[name].Default.(int)
		if ok {
			return defaultValue, nil
		}

		return 0, QueryParamNotFoundError{ParamName: name}
	}

	i, err := strconv.Atoi(param)
	if err != nil {
		return 0, QueryParamInvalidTypeError{
			ParamName:    name,
			ParamValue:   param,
			ExpectedType: "int",
			Err:          err,
		}
	}

	return i, nil
}

// QueryParamInt returns the query parameter with the given name as an int.
// If it does not exist, it returns the default value declared in the OpenAPI spec.
// For example, if the query parameter is declared as:
//
//	fuego.Get(s, "/test", myController,
//	  option.QueryInt("page", "Page number", param.Default(1))
//	)
//
// and the query parameter does not exist, it will return 1.
// If the query parameter does not exist and there is no default value, or if it is not an int, it returns 0.
func (c contextNoBodyImpl) QueryParamInt(name string) int {
	param, err := c.QueryParamIntErr(name)
	if err != nil {
		return 0
	}

	return param
}

// QueryParamBool returns the query parameter with the given name as a bool.
// If the query parameter does not exist or is not a bool, it returns the default value declared in the OpenAPI spec.
// For example, if the query parameter is declared as:
//
//	fuego.Get(s, "/test", myController,
//	  option.QueryBool("is_ok", "Is OK?", param.Default(true))
//	)
//
// and the query parameter does not exist in the HTTP request, it will return true.
// Accepted values are defined as [strconv.ParseBool]
func (c contextNoBodyImpl) QueryParamBoolErr(name string) (bool, error) {
	param := c.QueryParam(name)
	if param == "" {
		defaultValue, ok := c.params[name].Default.(bool)
		if ok {
			return defaultValue, nil
		}

		return false, QueryParamNotFoundError{ParamName: name}
	}

	b, err := strconv.ParseBool(param)
	if err != nil {
		return false, QueryParamInvalidTypeError{
			ParamName:    name,
			ParamValue:   param,
			ExpectedType: "bool",
			Err:          err,
		}
	}
	return b, nil
}

// QueryParamBool returns the query parameter with the given name as a bool.
// If the query parameter does not exist or is not a bool, it returns false.
// Accepted values are defined as [strconv.ParseBool]
// Example:
//
//	fuego.Get(s, "/test", myController,
//	  option.QueryBool("is_ok", "Is OK?", param.Default(true))
//	)
//
// and the query parameter does not exist in the HTTP request, it will return true.
func (c contextNoBodyImpl) QueryParamBool(name string) bool {
	param, err := c.QueryParamBoolErr(name)
	if err != nil {
		return false
	}

	return param
}

func (c contextNoBodyImpl) MainLang() string {
	return strings.Split(c.MainLocale(), "-")[0]
}

func (c contextNoBodyImpl) MainLocale() string {
	return strings.Split(c.Req.Header.Get("Accept-Language"), ",")[0]
}

// Request returns the HTTP request.
func (c contextNoBodyImpl) Request() *http.Request {
	return c.Req
}

// Response returns the HTTP response writer.
func (c contextNoBodyImpl) Response() http.ResponseWriter {
	return c.Res
}

// MustBody works like Body, but panics if there is an error.
func (c *contextWithBodyImpl[B]) MustBody() B {
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
func (c *contextWithBodyImpl[B]) Body() (B, error) {
	if c.body != nil {
		return *c.body, nil
	}

	body, err := body[B](c.contextNoBodyImpl)
	c.body = &body
	return body, err
}

func body[B any](c contextNoBodyImpl) (B, error) {
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

	c.Res.Header().Add("Server-Timing", Timing{"deserialize", time.Since(timeDeserialize), "controller > deserialize"}.String())

	return body, err
}
