package fuego

import (
	"errors"
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
	return &OpenAPI{
		description:            &desc,
		generator:              openapi3gen.NewGenerator(),
		globalOpenAPIResponses: []openAPIResponse{},
		Config:                 defaultOpenAPIConfig,
	}
}

// OpenAPI holds the OpenAPI OpenAPIDescription (OAD) and OpenAPI capabilities.
type OpenAPI struct {
	Config OpenAPIConfig

	description            *openapi3.T
	generator              *openapi3gen.Generator
	globalOpenAPIResponses []openAPIResponse
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

type OpenAPIServable interface {
	SpecHandler(e *Engine)
	UIHandler(e *Engine)
}

func (e *Engine) RegisterOpenAPIRoutes(o OpenAPIServable) {
	if e.OpenAPI.Config.Disabled {
		return
	}
	o.SpecHandler(e)

	if e.OpenAPI.Config.DisableSwaggerUI {
		return
	}
	o.UIHandler(e)
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

// RegisterParams registers the parameters of a given type to an OpenAPI operation.
// It inspects the fields of the provided struct, looking for "header" tags, and creates
// OpenAPI parameters for each tagged field.
func (route *RouteWithParams[Params, ResponseBody, RequestBody]) RegisterParams() error {
	if route.Operation == nil {
		route.Operation = openapi3.NewOperation()
	}
	params := *new(Params)
	typeOfParams := reflect.TypeOf(params)
	if typeOfParams == nil {
		return errors.New("params cannot be nil")
	}
	if typeOfParams.Kind() == reflect.Ptr {
		typeOfParams = typeOfParams.Elem()
	}

	if typeOfParams.Kind() == reflect.Struct {
		for i := range typeOfParams.NumField() {
			field := typeOfParams.Field(i)
			if headerKey, ok := field.Tag.Lookup("header"); ok {
				OptionHeader(headerKey, "string")(&route.BaseRoute)
			}
			if queryKey, ok := field.Tag.Lookup("query"); ok {
				OptionQuery(queryKey, "string")(&route.BaseRoute)
			}
			if cookieKey, ok := field.Tag.Lookup("cookie"); ok {
				OptionCookie(cookieKey, "string")(&route.BaseRoute)
			}
		}
	}

	return nil
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
func (openAPI *OpenAPI) getOrCreateSchema(key string, v any) *openapi3.Schema {
	schemaRef, ok := openAPI.Description().Components.Schemas[key]
	if !ok {
		schemaRef = openAPI.createSchema(key, v)
	}
	return schemaRef.Value
}

// createSchema is used to create a new schema and add it to the OpenAPI spec.
// Relies on the openapi3gen package to generate the schema, and adds custom struct tags.
func (openAPI *OpenAPI) createSchema(key string, v any) *openapi3.SchemaRef {
	schemaRef, err := openAPI.Generator().NewSchemaRefForValue(v, openAPI.Description().Components.Schemas)
	if err != nil {
		slog.Error("Error generating schema", "key", key, "error", err)
	}
	schemaRef.Value.Description = key + " schema"

	descriptionable, ok := v.(OpenAPIDescriptioner)
	if ok {
		schemaRef.Value.Description = descriptionable.Description()
	}

	parseStructTags(reflect.TypeOf(v), schemaRef)

	openAPI.Description().Components.Schemas[key] = schemaRef

	return schemaRef
}

// parseStructTags parses struct tags and modifies the schema accordingly.
// t must be a struct type.
// It adds the following struct tags (tag => OpenAPI schema field):
// - description => description
// - example => example
// - json => nullable (if contains omitempty)
// - validate:
//   - required => required
//   - min=1 => min=1 (for integers)
//   - min=1 => minLength=1 (for strings)
//   - max=100 => max=100 (for integers)
//   - max=100 => maxLength=100 (for strings)
func parseStructTags(t reflect.Type, schemaRef *openapi3.SchemaRef) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	schemaRef.Value.Required = []string{}

	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			fieldType := field.Type
			parseStructTags(fieldType, schemaRef)
			continue
		}

		jsonFieldName := field.Tag.Get("json")
		jsonFieldName = strings.Split(jsonFieldName, ",")[0] // remove omitempty, etc
		if jsonFieldName == "-" {
			continue
		}
		if jsonFieldName == "" {
			jsonFieldName = field.Name
		}

		property := schemaRef.Value.Properties[jsonFieldName]
		if property == nil {
			slog.Warn("Property not found in schema", "property", jsonFieldName)
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			parseStructTags(field.Type, property)
		}
		propertyCopy := *property
		propertyValue := *propertyCopy.Value

		// Example
		example, ok := field.Tag.Lookup("example")
		if ok {
			propertyValue.Example = example
			if propertyValue.Type.Is(openapi3.TypeInteger) {
				exNum, err := strconv.Atoi(example)
				if err != nil {
					slog.Warn("Example might be incorrect (should be integer)", "error", err)
				}
				propertyValue.Example = exNum
			}
		}

		// Validation
		validateTag, ok := field.Tag.Lookup("validate")
		validateTags := strings.Split(validateTag, ",")
		if ok && slices.Contains(validateTags, "required") {
			schemaRef.Value.Required = append(schemaRef.Value.Required, jsonFieldName)
		}
		for _, validateTag := range validateTags {
			if strings.HasPrefix(validateTag, "min=") {
				minValue, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
				if err != nil {
					slog.Warn("Min might be incorrect (should be integer)", "error", err)
				}

				if propertyValue.Type.Is(openapi3.TypeInteger) {
					minPtr := float64(minValue)
					propertyValue.Min = &minPtr
				} else if propertyValue.Type.Is(openapi3.TypeString) {
					//nolint:gosec // disable G115
					propertyValue.MinLength = uint64(minValue)
				}
			}
			if strings.HasPrefix(validateTag, "max=") {
				maxValue, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
				if err != nil {
					slog.Warn("Max might be incorrect (should be integer)", "error", err)
				}
				if propertyValue.Type.Is(openapi3.TypeInteger) {
					maxPtr := float64(maxValue)
					propertyValue.Max = &maxPtr
				} else if propertyValue.Type.Is(openapi3.TypeString) {
					//nolint:gosec // disable G115
					maxPtr := uint64(maxValue)
					propertyValue.MaxLength = &maxPtr
				}
			}
		}

		// Description
		description, ok := field.Tag.Lookup("description")
		if ok {
			propertyValue.Description = description
		}
		jsonTag, ok := field.Tag.Lookup("json")
		if ok {
			if strings.Contains(jsonTag, ",omitempty") {
				propertyValue.Nullable = true
			}
		}
		propertyCopy.Value = &propertyValue

		schemaRef.Value.Properties[jsonFieldName] = &propertyCopy
	}
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
