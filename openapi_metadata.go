package fuego

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	metadataParserExample     = "exampleParser"
	metadataParserValidation  = "validationParser"
	metadataParserDescription = "descriptionParser"
	metadataParserXML         = "XMLParser"
	metadataParserJSON        = "JSONParser"
)

var DefaultParsers = []MetadataParserEntry{
	{Name: metadataParserExample, Parser: MetadataParserExample},
	{Name: metadataParserValidation, Parser: MetadataParserValidation},
	{Name: metadataParserDescription, Parser: MetadataParserDescription},
	{Name: metadataParserXML, Parser: MetadataParserXML},
	{Name: metadataParserJSON, Parser: MetadataParserJSON},
}

type MetadataParserParams struct {
	// Reflective information of a struct field
	Field reflect.StructField
	// Name of the field
	FieldName string
	// OpenAPI schema of the field
	Property *openapi3.Schema
	// Reference to the OpenAPI schema
	SchemaRef *openapi3.SchemaRef
	// Additional metadata as key-value pairs
	Additional map[string]interface{}
}

type MetadataParserFunction func(params MetadataParserParams)

type MetadataParserEntry struct {
	Name   string
	Parser MetadataParserFunction
}

type MetadataParsers struct {
	registeredParsers []MetadataParserEntry
	parserLock        sync.Mutex
	registeredNames   map[string]bool
}

// NewMetadataParsers returns a new MetadataParsers instance with empty registeredParsers
// slice and registeredNames map.
func NewMetadataParsers() *MetadataParsers {
	return &MetadataParsers{
		registeredParsers: []MetadataParserEntry{},
		registeredNames:   make(map[string]bool),
	}
}

// GetRegisteredParsers returns a slice of all currently registered metadata parsers.
// This method ensures thread-safe access to the registeredParsers slice by using a mutex lock.
func (mp *MetadataParsers) GetRegisteredParsers() []MetadataParserEntry {
	mp.parserLock.Lock()
	defer mp.parserLock.Unlock()

	return mp.registeredParsers
}

// Reset clears all registered metadata parsers from the slice and resets the
// registeredNames map as empty. This method ensures thread-safe access to the
// registeredParsers slice and registeredNames map by using a mutex lock.
func (mp *MetadataParsers) Reset() {
	mp.parserLock.Lock()
	defer mp.parserLock.Unlock()

	mp.registeredParsers = []MetadataParserEntry{}
	mp.registeredNames = make(map[string]bool)
}

// Initialize initializes the metadata parsers with the given customParsers.
// It will append the customParsers to the existing DefaultParsers. If a parser with the
// same name is already registered, it will be skipped.
func (mp *MetadataParsers) Initialize(customParsers []MetadataParserEntry) {
	mp.parserLock.Lock()
	defer mp.parserLock.Unlock()

	mp.registeredParsers = []MetadataParserEntry{}
	mp.registeredNames = make(map[string]bool)

	var parsersToRegister = customParsers

	for _, entry := range parsersToRegister {
		if _, exists := mp.registeredNames[entry.Name]; exists {
			slog.Warn("Parser already registered", "name", entry.Name)
			continue
		}
		mp.registeredNames[entry.Name] = true
		mp.registeredParsers = append(mp.registeredParsers, entry)
	}
}

// RegisterMetadataParser registers a new metadata parser with the given name, parser
// function, and position string. The position string can be one of the following:
// "prepend", "append", "before", or "after". If the position is "before" or "after", you
// must also provide the name of the parser that you want to place the new parser
// before or after. If the relative parser is not found, an error is returned. If the
// parser is already registered, the function does nothing and returns nil. The
// function is thread-safe and ensures that the registeredParsers slice and
// registeredNames map are accessed in a thread-safe manner.
func (mp *MetadataParsers) RegisterMetadataParser(name string, parser MetadataParserFunction, position string, relativeTo string) error {
	validPositions := map[string]bool{"prepend": true, "append": true, "before": true, "after": true}
	if !validPositions[position] {
		return fmt.Errorf("Invalid position. Use 'prepend', 'append', 'before', or 'after'")
	}
	if (position == "before" || position == "after") && mp.findParserIndex(relativeTo) == -1 {
		return fmt.Errorf("Relative parser '%s' not found", relativeTo)
	}

	mp.parserLock.Lock()
	defer mp.parserLock.Unlock()

	for _, entry := range mp.registeredParsers {
		if entry.Name == name {
			// Remove the existing parser before re-registering
			mp.remove(entry)
			break
		}
	}

	newEntry := MetadataParserEntry{Name: name, Parser: parser}

	var err error
	switch position {
	case "prepend":
		mp.prepend(newEntry)
		slog.Debug("Parser registered at beginning", "name", name)
	case "append":
		mp.append(newEntry)
		slog.Debug("Parser registered at end", "name", name)
	case "before":
		err = mp.insertBefore(newEntry, relativeTo)
		if err != nil {
			slog.Error("Error registering parser", "error", err)
			return err
		}
		slog.Debug("Parser registered", "name", name, "position", "before", "relativeTo", relativeTo)
	case "after":
		err = mp.insertAfter(newEntry, relativeTo)
		if err != nil {
			slog.Error("Error registering parser", "error", err)
			return err
		}
		slog.Debug("Parser registered", "name", name, "position", "after", "relativeTo", relativeTo)
	}
	return nil
}

func (mp *MetadataParsers) prepend(entry MetadataParserEntry) {
	mp.registeredParsers = append([]MetadataParserEntry{entry}, mp.registeredParsers...)
}

func (mp *MetadataParsers) append(entry MetadataParserEntry) {
	mp.registeredParsers = append(mp.registeredParsers, entry)
}

func (mp *MetadataParsers) insertBefore(entry MetadataParserEntry, relativeTo string) error {
	index := mp.findParserIndex(relativeTo)
	if index == -1 {
		return fmt.Errorf("Relative parser '%s' not found", relativeTo)
	}
	mp.registeredParsers = append(mp.registeredParsers[:index], append([]MetadataParserEntry{entry}, mp.registeredParsers[index:]...)...)
	return nil
}

func (mp *MetadataParsers) insertAfter(entry MetadataParserEntry, relativeTo string) error {
	index := mp.findParserIndex(relativeTo)
	if index == -1 {
		return fmt.Errorf("Relative parser '%s' not found", relativeTo)
	}
	offset := 1
	mp.registeredParsers = append(mp.registeredParsers[:index+offset], append([]MetadataParserEntry{entry}, mp.registeredParsers[index+offset:]...)...)
	return nil
}

func (mp *MetadataParsers) remove(entry MetadataParserEntry) error {
	for i, e := range mp.registeredParsers {
		if e.Name == entry.Name {
			// Remove the entry by slicing out the matched element
			mp.registeredParsers = append(mp.registeredParsers[:i], mp.registeredParsers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("parser '%s' not found", entry.Name)
}

func (mp *MetadataParsers) findParserIndex(name string) int {
	for i, entry := range mp.registeredParsers {
		if entry.Name == name {
			return i
		}
	}
	return -1
}

// ParseStructTags iterates over the struct tags of the given type and calls each of the registered metadata parsers.
// The metadata parsers are called with the field, field name, property, and schema ref as arguments.
// The metadata parsers can modify the property in place.
// The function is called recursively if the field type is a struct.
// The function returns if the type is not a struct or if the field is anonymous.
func (mp *MetadataParsers) ParseStructTags(t reflect.Type, schemaRef *openapi3.SchemaRef) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	if schemaRef.Value.Properties == nil {
		schemaRef.Value.Properties = make(map[string]*openapi3.SchemaRef)
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Anonymous || (field.Name == "XMLName" && field.Type == reflect.TypeOf(xml.Name{})) {
			continue
		}

		fieldName := field.Tag.Get("json")
		fieldName = strings.Split(fieldName, ",")[0]
		if fieldName == "-" || fieldName == "" {
			fieldName = field.Name
		}

		if _, exists := schemaRef.Value.Properties[fieldName]; !exists {
			schemaRef.Value.Properties[fieldName] = &openapi3.SchemaRef{
				Value: &openapi3.Schema{},
			}
		}

		property := schemaRef.Value.Properties[fieldName]

		if field.Type.Kind() == reflect.Struct || (field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct) {
			mp.ParseStructTags(field.Type, property)
		}

		params := MetadataParserParams{
			Field:     field,
			FieldName: fieldName,
			Property:  property.Value,
			SchemaRef: schemaRef,
		}

		for _, entry := range mp.registeredParsers {
			entry.Parser(params)
		}
	}
}

// ExampleMetadataParser extracts the "example" tag from a struct field and assigns
// it to the Example property of the OpenAPI schema. If the field's type is an integer,
// it attempts to convert the example value to an integer, logging a warning if conversion fails.
func MetadataParserExample(params MetadataParserParams) {
	if exampleTag, ok := params.Field.Tag.Lookup("example"); ok {
		params.Property.Example = exampleTag
		if params.Property.Type.Is(openapi3.TypeInteger) {
			exNum, err := strconv.Atoi(exampleTag)
			if err != nil {
				slog.Warn("Example might be incorrect (should be integer)", "error", err)
			}
			params.Property.Example = exNum
		}
	}
}

// ValidationMetadataParser extracts the "validate" tag from a struct field and assigns
// any validation criteria to the OpenAPI schema. It supports the following validation
// criteria:
//
// - required: adds the field to the Required list on the schema
// - min=<integer>: sets the minimum value for the field in the schema
// - max=<integer>: sets the maximum value for the field in the schema
//
// The min and max values are interpreted as integers if the property type is an
// integer, and as string lengths if the property type is a string.
func MetadataParserValidation(params MetadataParserParams) {
	validateTag, ok := params.Field.Tag.Lookup("validate")
	validateTags := strings.Split(validateTag, ",")
	if ok && slices.Contains(validateTags, "required") {
		params.SchemaRef.Value.Required = append(params.SchemaRef.Value.Required, params.FieldName)
	}
	for _, validateTag := range validateTags {
		if strings.HasPrefix(validateTag, "min=") {
			min, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
			if err != nil {
				slog.Warn("Min might be incorrect (should be integer)", "error", err)
			}

			if params.Property.Type.Is(openapi3.TypeInteger) {
				minPtr := float64(min)
				params.Property.Min = &minPtr
			} else if params.Property.Type.Is(openapi3.TypeString) {
				//nolint:gosec // disable G115
				params.Property.MinLength = uint64(min)
			}
		}
		if strings.HasPrefix(validateTag, "max=") {
			max, err := strconv.Atoi(strings.Split(validateTag, "=")[1])
			if err != nil {
				slog.Warn("Max might be incorrect (should be integer)", "error", err)
			}
			if params.Property.Type.Is(openapi3.TypeInteger) {
				maxPtr := float64(max)
				params.Property.Max = &maxPtr
			} else if params.Property.Type.Is(openapi3.TypeString) {
				//nolint:gosec // disable G115
				maxPtr := uint64(max)
				params.Property.MaxLength = &maxPtr
			}
		}
	}
}

// DescriptionMetadataParser extracts the "description" tag from a struct field and assigns
// it to the Description property of the OpenAPI schema.
func MetadataParserDescription(params MetadataParserParams) {
	description := params.Field.Tag.Get("description")
	params.Property.Description = description
}

// XMLMetadataParser extracts the "xml" tag from a struct field and sets the XML
// representation of the OpenAPI schema. If the xml tag is "-", the function returns
// without making any changes. The XML field name is determined from the tag or defaults
// to the field's name. The function also checks for additional XML attributes such as
// "attr" and "wrapped" to set the corresponding properties in the OpenAPI XML schema.
func MetadataParserXML(params MetadataParserParams) {
	xmlField := params.Field.Tag.Get("xml")
	if xmlField == "-" || xmlField == "" {
		return
	}

	if params.Property == nil {
		params.Property = &openapi3.Schema{
			XML: &openapi3.XML{
				Attribute: false,
			},
		}
	}

	xmlFieldName := strings.Split(xmlField, ",")[0]
	if xmlFieldName == "-" {
		return
	}
	if xmlFieldName != "" {
		params.Property.XML = &openapi3.XML{
			Name: xmlFieldName,
		}
	}
	xmlFields := strings.Split(xmlField, ",")
	if slices.Contains(xmlFields, "attr") {
		params.Property.XML.Attribute = true
	}
	if slices.Contains(xmlFields, "wrapped") {
		params.Property.XML.Wrapped = true
	}

	if params.Field.Type.Kind() == reflect.Struct {
		if params.Property.Properties == nil {
			params.Property.Properties = make(map[string]*openapi3.SchemaRef)
		}
		processXMLStructChildren(params)
	} else if params.Field.Type.Kind() == reflect.Slice {
		elemType := params.Field.Type.Elem()
		if elemType.Kind() == reflect.Struct {
			params.Property = openapi3.NewArraySchema()
			params.Property.Items = &openapi3.SchemaRef{
				Value: &openapi3.Schema{},
			}

			processXMLStructChildren(MetadataParserParams{
				Field:     reflect.StructField{Type: elemType},
				Property:  params.Property.Items.Value,
				SchemaRef: params.SchemaRef,
			})
		}
	}
}

// processStructChildren processes the fields of a struct type and updates the
// OpenAPI schema properties map. For each field in the struct, it retrieves the
// "json" tag to determine the field name and adds a corresponding schema
// reference to the properties map if it doesn't already exist. The function
// then creates a MetadataParserParams object for the child field and recursively
// processes each child field using the MetadataParserXML function to apply XML
// parsing logic.
func processXMLStructChildren(params MetadataParserParams) {
	t := params.Field.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if params.Property.Properties == nil {
		params.Property.Properties = make(map[string]*openapi3.SchemaRef)
	}

	for i := 0; i < t.NumField(); i++ {
		childField := t.Field(i)
		childName := childField.Tag.Get("json")
		childName = strings.Split(childName, ",")[0]
		if childName == "-" || childName == "" {
			childName = childField.Name
		}

		if _, exists := params.Property.Properties[childName]; !exists {
			params.Property.Properties[childName] = &openapi3.SchemaRef{
				Value: &openapi3.Schema{},
			}
		}

		childProperty := params.Property.Properties[childName]

		childParams := MetadataParserParams{
			Field:     childField,
			FieldName: childName,
			Property:  childProperty.Value,
			SchemaRef: params.SchemaRef,
		}

		MetadataParserXML(childParams)
	}
}

// JSONMetadataParser sets the Nullable property of the OpenAPI schema based on the "json" tag.
// If the "json" tag contains ",omitempty", the Nullable property is set to true.
func MetadataParserJSON(params MetadataParserParams) {
	jsonTag, ok := params.Field.Tag.Lookup("json")
	if ok {
		if strings.Contains(jsonTag, ",omitempty") {
			params.Property.Nullable = true
		}
	}
}
