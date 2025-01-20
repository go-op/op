package fuego

import (
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
)

func NewOpenAPI() *OpenAPI {
	desc := NewOpenApiSpec()

	mp := NewMetadataParsers()
	mp.Initialize(DefaultParsers)

	return &OpenAPI{
		description:            &desc,
		generator:              openapi3gen.NewGenerator(),
		globalOpenAPIResponses: []openAPIResponse{},
		metadataParsers:        mp,
	}
}

// Holds the OpenAPI OpenAPIDescription (OAD) and OpenAPI capabilities.
type OpenAPI struct {
	description            *openapi3.T
	generator              *openapi3gen.Generator
	globalOpenAPIResponses []openAPIResponse
	metadataParsers        *MetadataParsers
}

func (openAPI *OpenAPI) Description() *openapi3.T {
	return openAPI.description
}

func (openAPI *OpenAPI) Generator() *openapi3gen.Generator {
	return openAPI.generator
}

// Compute the tags to declare at the root of the OpenAPI spec from the tags declared in the operations.
func (openAPI *OpenAPI) computeTags() {
	for _, pathItem := range openAPI.Description().Paths.Map() {
		for _, op := range pathItem.Operations() {
			for _, tag := range op.Tags {
				if openAPI.Description().Tags.Get(tag) == nil {
					openAPI.Description().Tags = append(openAPI.Description().Tags, &openapi3.Tag{
						Name: tag,
					})
				}
			}
		}
	}

	// Make sure tags are sorted
	slices.SortFunc(openAPI.Description().Tags, func(a, b *openapi3.Tag) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func NewOpenApiSpec() openapi3.T {
	info := &openapi3.Info{
		Title:       "OpenAPI",
		Description: openapiDescription,
		Version:     "0.0.1",
	}
	spec := openapi3.T{
		OpenAPI:  "3.1.0",
		Info:     info,
		Paths:    &openapi3.Paths{},
		Servers:  []*openapi3.Server{},
		Security: openapi3.SecurityRequirements{},
		Components: &openapi3.Components{
			Schemas:       make(map[string]*openapi3.SchemaRef),
			RequestBodies: make(map[string]*openapi3.RequestBodyRef),
			Responses:     make(map[string]*openapi3.ResponseRef),
		},
	}
	return spec
}

// Hide prevents the routes in this server or group from being included in the OpenAPI spec.
// Deprecated: Please use [OptionHide] with [WithRouteOptions]
func (s *Server) Hide() *Server {
	WithRouteOptions(
		OptionHide(),
	)(s)
	return s
}

// Show allows displaying the routes. Activated by default so useless in most cases,
// but this can be useful if you deactivated the parent group.
// Deprecated: Please use [OptionShow] with [WithRouteOptions]
func (s *Server) Show() *Server {
	WithRouteOptions(
		OptionShow(),
	)(s)
	return s
}

// OutputOpenAPISpec takes the OpenAPI spec and outputs it to a JSON file and/or serves it on a URL.
// Also serves a Swagger UI.
// To modify its behavior, use the [WithOpenAPIConfig] option.
func (s *Server) OutputOpenAPISpec() openapi3.T {
	s.OpenAPI.Description().Servers = append(s.OpenAPI.Description().Servers, &openapi3.Server{
		URL:         s.url(),
		Description: "local server",
	})

	if !s.OpenAPIConfig.Disabled {
		s.registerOpenAPIRoutes(s.Engine.OutputOpenAPISpec())
	}

	return *s.OpenAPI.Description()
}

// Registers the routes to serve the OpenAPI spec and Swagger UI.
func (s *Server) registerOpenAPIRoutes(jsonSpec []byte) {
	GetStd(s, s.OpenAPIServerConfig.SpecURL, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonSpec)
	})
	s.printOpenAPIMessage(fmt.Sprintf("JSON spec: %s%s", s.url(), s.OpenAPIServerConfig.SpecURL))

	if s.OpenAPIServerConfig.DisableSwaggerUI {
		return
	}
	Registers(s.Engine, netHttpRouteRegisterer[any, any]{
		s: s,
		route: Route[any, any]{
			BaseRoute: BaseRoute{
				Method: http.MethodGet,
				Path:   s.OpenAPIServerConfig.SwaggerURL + "/",
			},
		},
		controller: s.OpenAPIServerConfig.UIHandler(s.OpenAPIServerConfig.SpecURL),
	})
	s.printOpenAPIMessage(fmt.Sprintf("OpenAPI UI: %s%s/index.html", s.url(), s.OpenAPIServerConfig.SwaggerURL))
}

func validateSpecURL(specURL string) bool {
	specURLRegexp := regexp.MustCompile(`^\/[\/a-zA-Z0-9\-\_]+(.json)$`)
	return specURLRegexp.MatchString(specURL)
}

func validateSwaggerURL(swaggerURL string) bool {
	swaggerURLRegexp := regexp.MustCompile(`^\/[\/a-zA-Z0-9\-\_]+[a-zA-Z0-9\-\_]$`)
	return swaggerURLRegexp.MatchString(swaggerURL)
}

// RegisterOpenAPIOperation registers the route to the OpenAPI description.
// Modifies the route's Operation.
func (route *Route[ResponseBody, RequestBody]) RegisterOpenAPIOperation(openapi *OpenAPI) error {
	if route.Hidden || route.Method == "" {
		return nil
	}

	operation, err := RegisterOpenAPIOperation(openapi, *route)
	route.Operation = operation
	return err
}

// RegisterOpenAPIOperation registers an OpenAPI operation.
//
// Deprecated: Use `(*Route[ResponseBody, RequestBody]).RegisterOpenAPIOperation` instead.
func RegisterOpenAPIOperation[T, B any](openapi *OpenAPI, route Route[T, B]) (*openapi3.Operation, error) {
	if route.Operation == nil {
		route.Operation = openapi3.NewOperation()
	}

	if route.FullName == "" {
		route.FullName = route.Path
	}

	route.GenerateDefaultDescription()

	if route.Operation.Summary == "" {
		route.Operation.Summary = route.NameFromNamespace(camelToHuman)
	}

	if route.Operation.OperationID == "" {
		route.GenerateDefaultOperationID()
	}

	// Request Body
	if route.Operation.RequestBody == nil {
		bodyTag := SchemaTagFromType(openapi, *new(B))

		if bodyTag.Name != "unknown-interface" {
			requestBody := newRequestBody[B](bodyTag, route.RequestContentTypes)

			// add request body to operation
			route.Operation.RequestBody = &openapi3.RequestBodyRef{
				Value: requestBody,
			}
		}
	}

	// Response - globals
	for _, openAPIGlobalResponse := range openapi.globalOpenAPIResponses {
		addResponseIfNotSet(
			openapi,
			route.Operation,
			openAPIGlobalResponse.Code,
			openAPIGlobalResponse.Description,
			openAPIGlobalResponse.Response,
		)
	}

	// Automatically add non-declared 200 (or other) Response
	if route.DefaultStatusCode == 0 {
		route.DefaultStatusCode = 200
	}
	defaultStatusCode := strconv.Itoa(route.DefaultStatusCode)
	responseDefault := route.Operation.Responses.Value(defaultStatusCode)
	if responseDefault == nil {
		response := openapi3.NewResponse().WithDescription(http.StatusText(route.DefaultStatusCode))
		route.Operation.AddResponse(route.DefaultStatusCode, response)
		responseDefault = route.Operation.Responses.Value(defaultStatusCode)
	}

	// Automatically add non-declared Content for 200 (or other) Response
	if responseDefault.Value.Content == nil {
		responseSchema := SchemaTagFromType(openapi, *new(T))
		content := openapi3.NewContentWithSchemaRef(&responseSchema.SchemaRef, []string{"application/json", "application/xml"})
		responseDefault.Value.WithContent(content)
	}

	// Automatically add non-declared Path parameters
	for _, pathParam := range parsePathParams(route.Path) {
		if exists := route.Operation.Parameters.GetByInAndName("path", pathParam); exists != nil {
			continue
		}
		parameter := openapi3.NewPathParameter(pathParam)
		parameter.Schema = openapi3.NewStringSchema().NewRef()
		if strings.HasSuffix(pathParam, "...") {
			parameter.Description += " (might contain slashes)"
		}

		route.Operation.AddParameter(parameter)
	}
	for _, params := range route.Operation.Parameters {
		if params.Value.In == "path" {
			if !strings.Contains(route.Path, "{"+params.Value.Name) {
				panic(fmt.Errorf("path parameter '%s' is not declared in the path", params.Value.Name))
			}
		}
	}

	openapi.Description().AddOperation(route.Path, route.Method, route.Operation)

	return route.Operation, nil
}

func newRequestBody[RequestBody any](tag SchemaTag, consumes []string) *openapi3.RequestBody {
	content := openapi3.NewContentWithSchemaRef(&tag.SchemaRef, consumes)
	return openapi3.NewRequestBody().
		WithRequired(true).
		WithDescription("Request body for " + reflect.TypeOf(*new(RequestBody)).String()).
		WithContent(content)
}

// SchemaTag is a struct that holds the name of the struct and the associated openapi3.SchemaRef
type SchemaTag struct {
	openapi3.SchemaRef
	Name string
}

func SchemaTagFromType(openapi *OpenAPI, v any) SchemaTag {
	if v == nil {
		// ensure we add unknown-interface to our schemas
		schema := openapi.getOrCreateSchema("unknown-interface", struct{}{})
		return SchemaTag{
			Name: "unknown-interface",
			SchemaRef: openapi3.SchemaRef{
				Ref:   "#/components/schemas/unknown-interface",
				Value: schema,
			},
		}
	}

	return dive(openapi, reflect.TypeOf(v), SchemaTag{}, 5)
}

// dive returns a schemaTag which includes the generated openapi3.SchemaRef and
// the name of the struct being passed in.
// If the type is a pointer, map, channel, function, or unsafe pointer,
// it will dive into the type and return the name of the type it points to.
// If the type is a slice or array type it will dive into the type as well as
// build and openapi3.Schema where Type is array and Ref is set to the proper
// components Schema
func dive(openapi *OpenAPI, t reflect.Type, tag SchemaTag, maxDepth int) SchemaTag {
	if maxDepth == 0 {
		return SchemaTag{
			Name: "default",
			SchemaRef: openapi3.SchemaRef{
				Ref: "#/components/schemas/default",
			},
		}
	}

	switch t.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return dive(openapi, t.Elem(), tag, maxDepth-1)

	case reflect.Slice, reflect.Array:
		item := dive(openapi, t.Elem(), tag, maxDepth-1)
		tag.Name = item.Name
		tag.Value = openapi3.NewArraySchema()
		tag.Value.Items = &item.SchemaRef
		return tag

	default:
		tag.Name = transformTypeName(t.Name())
		if t.Kind() == reflect.Struct && strings.HasPrefix(tag.Name, "DataOrTemplate") {
			return dive(openapi, t.Field(0).Type, tag, maxDepth-1)
		}
		tag.Ref = "#/components/schemas/" + tag.Name
		tag.Value = openapi.getOrCreateSchema(tag.Name, reflect.New(t).Interface())

		return tag
	}
}

// getOrCreateSchema is used to get a schema from the OpenAPI spec.
// If the schema does not exist, it will create a new schema and add it to the OpenAPI spec.
func (openapi *OpenAPI) getOrCreateSchema(key string, v any) *openapi3.Schema {
	schemaRef, ok := openapi.Description().Components.Schemas[key]
	if !ok {
		schemaRef = openapi.createSchema(key, v)
	}
	return schemaRef.Value
}

// createSchema is used to create a new schema and add it to the OpenAPI spec.
// Relies on the openapi3gen package to generate the schema, and adds custom struct tags.
func (openapi *OpenAPI) createSchema(key string, v any) *openapi3.SchemaRef {
	schemaRef, err := openapi.Generator().NewSchemaRefForValue(v, openapi.Description().Components.Schemas)
	if err != nil {
		slog.Error("Error generating schema", "key", key, "error", err)
	}
	schemaRef.Value.Description = key + " schema"

	descriptionable, ok := v.(OpenAPIDescriptioner)
	if ok {
		schemaRef.Value.Description = descriptionable.Description()
	}

	openapi.metadataParsers.ParseStructTags(reflect.TypeOf(v), schemaRef)

	openapi.Description().Components.Schemas[key] = schemaRef

	return schemaRef
}

type OpenAPIDescriptioner interface {
	Description() string
}

// Transform the type name to a more readable & valid OpenAPI 3 format.
// Useful for generics.
// Example: "BareSuccessResponse[github.com/go-fuego/fuego/examples/petstore/models.Pets]" -> "BareSuccessResponse_models.Pets"
func transformTypeName(s string) string {
	// Find the positions of the '[' and ']'
	start := strings.Index(s, "[")
	if start == -1 {
		return s
	}
	end := strings.Index(s, "]")
	if end == -1 {
		return s
	}

	prefix := s[:start]

	inside := s[start+1 : end]

	lastSlash := strings.LastIndex(inside, "/")
	if lastSlash != -1 {
		inside = inside[lastSlash+1:]
	}

	return prefix + "_" + inside
}
