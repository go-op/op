package fuego

import (
	"slices"

	"github.com/go-fuego/fuego/openapi3"
)

type OpenAPIParam struct {
	Required bool
	Example  string
	Type     string // "query", "header", "cookie"
}

type Route[ResponseBody any, RequestBody any] struct {
	operation *openapi3.Operation
}

func (r Route[ResponseBody, RequestBody]) Description(description string) Route[ResponseBody, RequestBody] {
	r.operation.Description = description
	return r
}

func (r Route[ResponseBody, RequestBody]) Summary(summary string) Route[ResponseBody, RequestBody] {
	r.operation.Summary = summary
	return r
}

// Overrides the operationID for the route.
func (r Route[ResponseBody, RequestBody]) OperationID(operationID string) Route[ResponseBody, RequestBody] {
	r.operation.OperationID = operationID
	return r
}

// Param registers a parameter for the route.
// The paramType can be "query", "header" or "cookie".
// [Cookie], [Header], [QueryParam] are shortcuts for Param.
func (r Route[ResponseBody, RequestBody]) Param(paramType, name, description string, params ...OpenAPIParam) Route[ResponseBody, RequestBody] {
	openapiParam := &openapi3.Parameter{
		Name:        name,
		Description: description,
		In:          paramType,
	}

	for _, param := range params {
		if param.Required {
			openapiParam.Required = param.Required
		}
	}

	r.operation.Parameters = append(r.operation.Parameters, openapiParam)

	return r
}

// Header registers a header parameter for the route.
func (r Route[ResponseBody, RequestBody]) Header(name, description string, params ...OpenAPIParam) Route[ResponseBody, RequestBody] {
	r.Param("header", name, description, params...)
	return r
}

// Cookie registers a cookie parameter for the route.
func (r Route[ResponseBody, RequestBody]) Cookie(name, description string, params ...OpenAPIParam) Route[ResponseBody, RequestBody] {
	r.Param("cookie", name, description, params...)
	return r
}

// QueryParam registers a query parameter for the route.
func (r Route[ResponseBody, RequestBody]) QueryParam(name, description string, params ...OpenAPIParam) Route[ResponseBody, RequestBody] {
	r.Param("query", name, description, params...)
	return r
}

// Replace the tags for the route.
// By default, the tag is the type of the response body.
func (r Route[ResponseBody, RequestBody]) Tags(tags ...string) Route[ResponseBody, RequestBody] {
	r.operation.Tags = tags
	return r
}

// AddTags adds tags to the route.
func (r Route[ResponseBody, RequestBody]) AddTags(tags ...string) Route[ResponseBody, RequestBody] {
	r.operation.Tags = append(r.operation.Tags, tags...)
	return r
}

// RemoveTags removes tags from the route.
func (r Route[ResponseBody, RequestBody]) RemoveTags(tags ...string) Route[ResponseBody, RequestBody] {
	for _, tag := range tags {
		for i, t := range r.operation.Tags {
			if t == tag {
				r.operation.Tags = slices.Delete(r.operation.Tags, i, i+1)
				break
			}
		}
	}
	return r
}

func (r Route[ResponseBody, RequestBody]) Deprecated() Route[ResponseBody, RequestBody] {
	r.operation.Deprecated = true
	return r
}
