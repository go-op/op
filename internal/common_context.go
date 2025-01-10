package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"
)

type OpenAPIParam struct {
	Name        string
	Description string

	// Default value for the parameter.
	// Type is checked at start-time.
	Default  any
	Example  string
	Examples map[string]any
	Type     ParamType

	// integer, string, bool
	GoType string

	// Status codes for which this parameter is required.
	// Only used for response parameters.
	// If empty, it is required for 200 status codes.
	StatusCodes []int

	Required bool
	Nullable bool
}

// Base context shared by all adaptors (net/http, gin, echo, etc...)
type CommonContext[B any] struct {
	CommonCtx context.Context

	UrlValues     url.Values
	OpenAPIParams map[string]OpenAPIParam // list of expected query parameters (declared in the OpenAPI spec)

	// default status code for the response
	DefaultStatusCode int
}

type ParamType string // Query, Header, Cookie

// GetOpenAPIParams returns the OpenAPI parameters declared in the OpenAPI spec.
func (c CommonContext[B]) GetOpenAPIParams() map[string]OpenAPIParam {
	return c.OpenAPIParams
}

func (c CommonContext[B]) Context() context.Context {
	return c.CommonCtx
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c CommonContext[B]) Deadline() (deadline time.Time, ok bool) {
	return c.Context().Deadline()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c CommonContext[B]) Done() <-chan struct{} {
	return c.Context().Done()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c CommonContext[B]) Err() error {
	return c.Context().Err()
}

// ContextNoBody implements the context interface via [net/http.Request.Context]
func (c CommonContext[B]) Value(key any) any {
	return c.Context().Value(key)
}

// QueryParams returns the query parameters of the request. It is a shortcut for c.Req.URL.Query().
func (c CommonContext[B]) QueryParams() url.Values {
	return c.UrlValues
}

// HasQueryParam returns true if the query parameter with the given name exists.
func (c CommonContext[B]) HasQueryParam(name string) bool {
	_, ok := c.UrlValues[name]
	return ok
}

// QueryParam returns the query parameter with the given name.
// If it does not exist, it returns an empty string, unless there is a default value declared in the OpenAPI spec.
//
// Example:
//
//	fuego.Get(s, "/test", myController,
//	  option.Query("name", "Name", param.Default("hey"))
//	)
func (c CommonContext[B]) QueryParam(name string) string {
	_, ok := c.OpenAPIParams[name]
	if !ok {
		slog.Warn("query parameter not expected in OpenAPI spec", "param", name, "expected_one_of", c.OpenAPIParams)
	}

	if !c.UrlValues.Has(name) {
		defaultValue, _ := c.OpenAPIParams[name].Default.(string)
		return defaultValue
	}
	return c.UrlValues.Get(name)
}

func (c CommonContext[B]) QueryParamIntErr(name string) (int, error) {
	param := c.QueryParam(name)
	if param == "" {
		defaultValue, ok := c.OpenAPIParams[name].Default.(int)
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

type QueryParamNotFoundError struct {
	ParamName string
}

func (e QueryParamNotFoundError) Error() string {
	return fmt.Errorf("param %s not found", e.ParamName).Error()
}

type QueryParamInvalidTypeError struct {
	Err          error
	ParamName    string
	ParamValue   string
	ExpectedType string
}

func (e QueryParamInvalidTypeError) Error() string {
	return fmt.Errorf("param %s=%s is not of type %s: %w", e.ParamName, e.ParamValue, e.ExpectedType, e.Err).Error()
}

// QueryParamsArr returns an slice of string from the given query parameter.
func (c CommonContext[B]) QueryParamArr(name string) []string {
	_, ok := c.OpenAPIParams[name]
	if !ok {
		slog.Warn("query parameter not expected in OpenAPI spec", "param", name)
	}
	return c.UrlValues[name]
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
func (c CommonContext[B]) QueryParamInt(name string) int {
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
func (c CommonContext[B]) QueryParamBoolErr(name string) (bool, error) {
	param := c.QueryParam(name)
	if param == "" {
		defaultValue, ok := c.OpenAPIParams[name].Default.(bool)
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
func (c CommonContext[B]) QueryParamBool(name string) bool {
	param, err := c.QueryParamBoolErr(name)
	if err != nil {
		return false
	}

	return param
}
