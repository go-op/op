package openapi3

import (
	"reflect"
	"strings"
	"time"
)

// ToSchema converts any Go type to an OpenAPI Schema
func ToSchema(v any) *Schema {
	if v == nil {
		return nil
	}

	s := Schema{
		Type:       "object",
		Properties: make(map[string]Schema),
	}

	value := reflect.ValueOf(v)

	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	if value.Kind() == reflect.Slice {
		s.Type = "array"
		itemType := value.Type().Elem()
		if itemType.Kind() == reflect.Ptr {
			itemType = itemType.Elem()
		}
		one := reflect.New(itemType)
		s.Items = ToSchema(one.Interface())
	}

	if _, isTime := value.Interface().(time.Time); isTime {
		s.Type = "string"
		s.Format = "date-time"
		s.Example = value.Interface().(time.Time).Format(time.RFC3339)
		return &s
	}

	if value.Kind() == reflect.Struct {
		// Iterate on fields with reflect
		for i := range value.NumField() {
			field := value.Field(i)
			fieldType := value.Type().Field(i)

			// If the field is a struct, we need to dive into it
			if field.Kind() == reflect.Struct {
				fieldName := fieldType.Tag.Get("json")
				if fieldName == "" {
					fieldName = fieldType.Name
				}
				s.Properties[fieldName] = *ToSchema(field.Interface())
			} else {
				// If the field is a basic type, we can just add it to the properties
				fieldTypeType := fieldType.Type.Name()
				format := fieldType.Tag.Get("format")
				if strings.Contains(fieldTypeType, "int") {
					fieldTypeType = "integer"
					if format != "" {
						format = fieldType.Type.Name()
					}
				} else if fieldTypeType == "bool" {
					fieldTypeType = "boolean"
				}
				fieldName := fieldType.Tag.Get("json")
				if fieldName == "" {
					fieldName = fieldType.Name
				}
				if strings.Contains(fieldType.Tag.Get("validate"), "required") {
					s.Required = append(s.Required, fieldName)
				}
				s.Properties[fieldName] = Schema{
					Type:    fieldTypeType,
					Example: fieldType.Tag.Get("example"),
					Format:  format,
				}
			}
		}
	}

	if !(value.Kind() == reflect.Struct || value.Kind() == reflect.Slice) {
		s.Type = value.Kind().String()
		if strings.Contains(s.Type, "int") {
			s.Type = "integer"
		} else if s.Type == "bool" {
			s.Type = "boolean"
		}
	}

	return &s
}
