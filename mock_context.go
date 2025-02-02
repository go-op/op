package fuego

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-fuego/fuego/internal"
)

// MockContext provides a framework-agnostic implementation of ContextWithBody
// for testing purposes. It allows testing controllers without depending on
// specific web frameworks like Gin or Echo.
type MockContext[B any] struct {
	internal.CommonContext[B]

	RequestBody B
	Headers     http.Header
	PathParams  map[string]string
	response    http.ResponseWriter
	request     *http.Request
	Cookies     map[string]*http.Cookie
}

// NewMockContext creates a new MockContext instance with the provided body
func NewMockContext[B any](body B) *MockContext[B] {
	return &MockContext[B]{
		CommonContext: internal.CommonContext[B]{
			CommonCtx:         context.Background(),
			UrlValues:         make(url.Values),
			OpenAPIParams:     make(map[string]internal.OpenAPIParam),
			DefaultStatusCode: http.StatusOK,
		},
		RequestBody: body,
		Headers:     make(http.Header),
		PathParams:  make(map[string]string),
		Cookies:     make(map[string]*http.Cookie),
	}
}

// NewMockContextNoBody creates a new MockContext suitable for a request & controller with no body
func NewMockContextNoBody() *MockContext[any] {
	return NewMockContext[any](nil)
}

var _ ContextWithBody[string] = &MockContext[string]{}

// Body returns the previously set body value
func (m *MockContext[B]) Body() (B, error) {
	return m.RequestBody, nil
}

// MustBody returns the body or panics if there's an error
func (m *MockContext[B]) MustBody() B {
	return m.RequestBody
}

// HasHeader checks if a header exists
func (m *MockContext[B]) HasHeader(key string) bool {
	_, exists := m.Headers[key]
	return exists
}

// HasCookie checks if a cookie exists
func (m *MockContext[B]) HasCookie(key string) bool {
	_, exists := m.Cookies[key]
	return exists
}

// Header returns the value of the specified header
func (m *MockContext[B]) Header(key string) string {
	return m.Headers.Get(key)
}

// SetHeader sets a header in the mock context
func (m *MockContext[B]) SetHeader(key, value string) {
	m.Headers.Set(key, value)
}

// PathParam returns a mock path parameter
func (m *MockContext[B]) PathParam(name string) string {
	return m.PathParams[name]
}

// Request returns the mock request
func (m *MockContext[B]) Request() *http.Request {
	return m.request
}

// Response returns the mock response writer
func (m *MockContext[B]) Response() http.ResponseWriter {
	return m.response
}

// SetStatus sets the response status code
func (m *MockContext[B]) SetStatus(code int) {
	if m.response != nil {
		m.response.WriteHeader(code)
	}
}

// Cookie returns a mock cookie
func (m *MockContext[B]) Cookie(name string) (*http.Cookie, error) {
	cookie, exists := m.Cookies[name]
	if !exists {
		return nil, http.ErrNoCookie
	}
	return cookie, nil
}

// SetCookie sets a cookie in the mock context
func (m *MockContext[B]) SetCookie(cookie http.Cookie) {
	m.Cookies[cookie.Name] = &cookie
}

// MainLang returns the main language from Accept-Language header
func (m *MockContext[B]) MainLang() string {
	lang := m.Headers.Get("Accept-Language")
	if lang == "" {
		return ""
	}
	return strings.Split(strings.Split(lang, ",")[0], "-")[0]
}

// MainLocale returns the main locale from Accept-Language header
func (m *MockContext[B]) MainLocale() string {
	return m.Headers.Get("Accept-Language")
}

// Redirect returns a redirect response
func (m *MockContext[B]) Redirect(code int, url string) (any, error) {
	if m.response != nil {
		http.Redirect(m.response, m.request, url, code)
	}
	return nil, nil
}

// Render is a mock implementation that does nothing
func (m *MockContext[B]) Render(templateToExecute string, data any, templateGlobsToOverride ...string) (CtxRenderer, error) {
	panic("not implemented")
}

// SetQueryParam adds a query parameter to the mock context with OpenAPI validation
func (m *MockContext[B]) SetQueryParam(name, value string) *MockContext[B] {
	param := OpenAPIParam{
		Name:   name,
		GoType: "string",
		Type:   "query",
	}

	m.CommonContext.OpenAPIParams[name] = param
	m.CommonContext.UrlValues.Set(name, value)
	return m
}

// SetQueryParamInt adds an integer query parameter to the mock context with OpenAPI validation
func (m *MockContext[B]) SetQueryParamInt(name string, value int) *MockContext[B] {
	param := OpenAPIParam{
		Name:   name,
		GoType: "integer",
		Type:   "query",
	}

	m.CommonContext.OpenAPIParams[name] = param
	m.CommonContext.UrlValues.Set(name, fmt.Sprintf("%d", value))
	return m
}

// SetQueryParamBool adds a boolean query parameter to the mock context with OpenAPI validation
func (m *MockContext[B]) SetQueryParamBool(name string, value bool) *MockContext[B] {
	param := OpenAPIParam{
		Name:   name,
		GoType: "boolean",
		Type:   "query",
	}

	m.CommonContext.OpenAPIParams[name] = param
	m.CommonContext.UrlValues.Set(name, fmt.Sprintf("%t", value))
	return m
}
