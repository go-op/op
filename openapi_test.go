package fuego

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

type MyStruct struct {
	B string `json:"b"`
	C int    `json:"c"`
	D bool   `json:"d"`
}

type MyOutputStruct struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type testCaseForTagType[V any] struct {
	name        string
	description string
	inputType   V

	expectedTagValue     string
	expectedTagValueType *openapi3.Types
}

func Test_tagFromType(t *testing.T) {
	s := NewServer()
	type DeeplyNested *[]MyStruct
	type MoreDeeplyNested *[]DeeplyNested

	tcs := []testCaseForTagType[any]{
		{
			name:        "unknown_interface",
			description: "behind any interface",
			inputType:   *new(any),

			expectedTagValue: "unknown-interface",
		},
		{
			name:        "simple_struct",
			description: "basic struct",
			inputType:   MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "is_pointer",
			description: "",
			inputType:   &MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "is_array",
			description: "",
			inputType:   []MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "is_reference_to_array",
			description: "",
			inputType:   &[]MyStruct{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "is_deeply_nested",
			description: "behind 4 pointers",
			inputType:   new(DeeplyNested),

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "5_pointers",
			description: "behind 5 pointers",
			inputType:   *new(MoreDeeplyNested),

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "6_pointers",
			description: "behind 6 pointers",
			inputType:   new(MoreDeeplyNested),

			expectedTagValue:     "default",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "7_pointers",
			description: "behind 7 pointers",
			inputType:   []*MoreDeeplyNested{},

			expectedTagValue: "default",
		},
		{
			name:        "detecting_string",
			description: "",
			inputType:   "string",

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"string"},
		},
		{
			name:        "new_string",
			description: "",
			inputType:   new(string),

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"string"},
		},
		{
			name:        "string_array",
			description: "",
			inputType:   []string{},

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "pointer_string_array",
			description: "",
			inputType:   &[]string{},

			expectedTagValue:     "string",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "DataOrTemplate",
			description: "",
			inputType:   DataOrTemplate[MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "ptr to DataOrTemplate",
			description: "",
			inputType:   &DataOrTemplate[MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"object"},
		},
		{
			name:        "DataOrTemplate of an array",
			description: "",
			inputType:   DataOrTemplate[[]MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of an array of ptr",
			description: "",
			inputType:   &DataOrTemplate[[]*MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of a ptr to an array",
			description: "",
			inputType:   &DataOrTemplate[*[]MyStruct]{},

			expectedTagValue:     "MyStruct",
			expectedTagValueType: &openapi3.Types{"array"},
		},
		{
			name:        "ptr to DataOrTemplate of a ptr to an array of ptr",
			description: "",
			inputType:   &DataOrTemplate[*[]*MyStruct]{},

			expectedTagValue:     "default",
			expectedTagValueType: &openapi3.Types{"array"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tag := SchemaTagFromType(s.OpenAPI, tc.inputType)
			require.Equal(t, tc.expectedTagValue, tag.Name, tc.description)
			if tc.expectedTagValueType != nil {
				require.NotNil(t, tag.Value)
				require.Equal(t, tc.expectedTagValueType, tag.Value.Type, tc.description)
			}
		})
	}
}

func TestServer_generateOpenAPI(t *testing.T) {
	s := NewServer()
	Get(s, "/", func(ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})
	Post(s, "/post", func(ContextWithBody[MyStruct]) ([]MyStruct, error) {
		return nil, nil
	})
	Get(s, "/post/{id}", func(ContextNoBody) (MyOutputStruct, error) {
		return MyOutputStruct{}, nil
	})
	Post(s, "/multidimensional/post", func(ContextWithBody[MyStruct]) ([][]MyStruct, error) {
		return nil, nil
	})
	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.NotNil(t, document.Paths.Find("/"))
	require.Nil(t, document.Paths.Find("/unknown"))
	require.NotNil(t, document.Paths.Find("/post"))
	require.Equal(t, document.Paths.Find("/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Ref, "#/components/schemas/MyStruct")
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Value.Type, &openapi3.Types{"array"})
	require.Equal(t, document.Paths.Find("/multidimensional/post").Post.Responses.Value("200").Value.Content["application/json"].Schema.Value.Items.Value.Items.Ref, "#/components/schemas/MyStruct")
	require.NotNil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200"))
	require.NotNil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"])
	require.Nil(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["unknown"])
	require.Equal(t, document.Paths.Find("/post/{id}").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["quantity"].Value.Type, &openapi3.Types{"integer"})

	t.Run("openapi doc is available through a route", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/swagger/openapi.json", nil)
		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
	})
}

func TestServer_OutputOpenApiSpec(t *testing.T) {
	docPath := "doc/openapi.json"
	t.Run("base", func(t *testing.T) {
		s := NewServer(
			WithOpenAPIConfig(
				OpenAPIConfig{
					JsonFilePath: docPath,
				},
			),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})

		document := s.OutputOpenAPISpec()
		require.NotNil(t, document)

		file, err := os.Open(docPath)
		require.NoError(t, err)
		require.NotNil(t, file)
		defer os.Remove(file.Name())
		require.Equal(t, 1, lineCounter(t, file))
	})
	t.Run("do not print file", func(t *testing.T) {
		s := NewServer(
			WithOpenAPIConfig(
				OpenAPIConfig{
					JsonFilePath:     docPath,
					DisableLocalSave: true,
				},
			),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})

		document := s.OutputOpenAPISpec()
		require.NotNil(t, document)

		file, err := os.Open(docPath)
		require.Error(t, err)
		require.Nil(t, file)
	})
	t.Run("swagger disabled", func(t *testing.T) {
		s := NewServer(
			WithOpenAPIConfig(
				OpenAPIConfig{
					JsonFilePath:     docPath,
					DisableLocalSave: true,
					DisableSwagger:   true,
				},
			),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})

		document := s.OutputOpenAPISpec()
		require.Len(t, document.Paths.Map(), 1)
		require.NotNil(t, document)

		file, err := os.Open(docPath)
		require.Error(t, err)
		require.Nil(t, file)
	})
	t.Run("pretty format json file", func(t *testing.T) {
		s := NewServer(
			WithOpenAPIConfig(
				OpenAPIConfig{
					JsonFilePath:     docPath,
					PrettyFormatJson: true,
				},
			),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})

		document := s.OutputOpenAPISpec()
		require.NotNil(t, document)

		file, err := os.Open(docPath)
		require.NoError(t, err)
		require.NotNil(t, file)
		defer os.Remove(file.Name())
		require.Greater(t, lineCounter(t, file), 1)
	})
}

func lineCounter(t *testing.T, r io.Reader) int {
	buf := make([]byte, 32*1024)
	count := 1
	lineSep := []byte{'\n'}

	c, err := r.Read(buf)
	require.NoError(t, err)
	count += bytes.Count(buf[:c], lineSep)
	return count
}

func BenchmarkRoutesRegistration(b *testing.B) {
	for range b.N {
		s := NewServer(
			WithoutLogger(),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})
		for j := 0; j < 100; j++ {
			Post(s, fmt.Sprintf("/post/%d", j), func(ContextWithBody[MyStruct]) ([]MyStruct, error) {
				return nil, nil
			})
		}
		for j := 0; j < 100; j++ {
			Get(s, fmt.Sprintf("/post/{id}/%d", j), func(ContextNoBody) (MyStruct, error) {
				return MyStruct{}, nil
			})
		}
	}
}

func BenchmarkServer_generateOpenAPI(b *testing.B) {
	for range b.N {
		s := NewServer(
			WithoutLogger(),
		)
		Get(s, "/", func(ContextNoBody) (MyStruct, error) {
			return MyStruct{}, nil
		})
		for j := 0; j < 100; j++ {
			Post(s, fmt.Sprintf("/post/%d", j), func(ContextWithBody[MyStruct]) ([]MyStruct, error) {
				return nil, nil
			})
		}
		for j := 0; j < 100; j++ {
			Get(s, fmt.Sprintf("/post/{id}/%d", j), func(ContextNoBody) (MyStruct, error) {
				return MyStruct{}, nil
			})
		}

		s.OutputOpenAPISpec()
	}
}

func TestValidateJsonSpecUrl(t *testing.T) {
	require.Equal(t, true, validateJsonSpecUrl("/path/to/jsonSpec.json"))
	require.Equal(t, true, validateJsonSpecUrl("/spec.json"))
	require.Equal(t, true, validateJsonSpecUrl("/path_/jsonSpec.json"))
	require.Equal(t, false, validateJsonSpecUrl("path/to/jsonSpec.json"))
	require.Equal(t, false, validateJsonSpecUrl("/path/to/jsonSpec"))
	require.Equal(t, false, validateJsonSpecUrl("/path/to/jsonSpec.jsn"))
}

func TestValidateSwaggerUrl(t *testing.T) {
	require.Equal(t, true, validateSwaggerUrl("/path/to/jsonSpec"))
	require.Equal(t, true, validateSwaggerUrl("/swagger"))
	require.Equal(t, true, validateSwaggerUrl("/Super-useful_swagger-2000"))
	require.Equal(t, true, validateSwaggerUrl("/Super-useful_swagger-"))
	require.Equal(t, true, validateSwaggerUrl("/Super-useful_swagger__"))
	require.Equal(t, true, validateSwaggerUrl("/Super-useful_swaggeR"))
	require.Equal(t, false, validateSwaggerUrl("/spec.json"))
	require.Equal(t, false, validateSwaggerUrl("/path_/swagger.json"))
	require.Equal(t, false, validateSwaggerUrl("path/to/jsonSpec."))
	require.Equal(t, false, validateSwaggerUrl("path/to/jsonSpec%"))
}

func TestLocalSave(t *testing.T) {
	s := NewServer()
	t.Run("with valid path", func(t *testing.T) {
		err := s.saveOpenAPIToFile("/tmp/jsonSpec.json", []byte("test"))
		require.NoError(t, err)

		// cleanup
		os.Remove("/tmp/jsonSpec.json")
	})

	t.Run("with invalid path", func(t *testing.T) {
		err := s.saveOpenAPIToFile("///jsonSpec.json", []byte("test"))
		require.Error(t, err)
	})
}

func TestAutoGroupTags(t *testing.T) {
	s := NewServer(
		WithOpenAPIConfig(OpenAPIConfig{
			DisableLocalSave: true,
			DisableSwagger:   true,
		}),
	)
	Get(s, "/a", func(ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	group := Group(s, "/group")
	Get(group, "/b", func(ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	subGroup := Group(group, "/subgroup")
	Get(subGroup, "/c", func(ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	otherGroup := Group(s, "/other")
	Get(otherGroup, "/d", func(ContextNoBody) (MyStruct, error) {
		return MyStruct{}, nil
	})

	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.Nil(t, document.Paths.Find("/a").Get.Tags)
	require.Equal(t, []string{"group"}, document.Paths.Find("/group/b").Get.Tags)
	require.Equal(t, []string{"group", "subgroup"}, document.Paths.Find("/group/subgroup/c").Get.Tags)
	require.Equal(t, []string{"other"}, document.Paths.Find("/other/d").Get.Tags)
}

func TestValidationTags(t *testing.T) {
	type MyType struct {
		Name string `json:"name" validate:"required,min=3,max=10" description:"Name of the user" example:"John"`
		Age  int    `json:"age" validate:"min=18,max=100" description:"Age of the user" example:"25"`
	}

	s := NewServer()
	Get(s, "/data", func(ContextNoBody) (MyType, error) {
		return MyType{}, nil
	})

	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)
	require.NotNil(t, document.Paths.Find("/data").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["name"].Value.Description)
	require.Equal(t, "Name of the user", document.Paths.Find("/data").Get.Responses.Value("200").Value.Content["application/json"].Schema.Value.Properties["name"].Value.Description)

	myTypeValue := document.Components.Schemas["MyType"].Value
	t.Logf("myType: %+v", myTypeValue)
	t.Logf("name: %+v", myTypeValue.Properties["name"])
	t.Logf("age: %+v", myTypeValue.Properties["age"])

	require.NotNil(t, myTypeValue.Properties["name"].Value.Description)
	require.Equal(t, "John", myTypeValue.Properties["name"].Value.Example)
	require.Equal(t, "Name of the user", myTypeValue.Properties["name"].Value.Description)
	var expected *float64
	require.Equal(t, expected, myTypeValue.Properties["name"].Value.Min)
	require.Equal(t, uint64(3), myTypeValue.Properties["name"].Value.MinLength)
	require.Equal(t, uint64(10), *myTypeValue.Properties["name"].Value.MaxLength)
	require.Equal(t, float64(18.0), *myTypeValue.Properties["age"].Value.Min)
	require.Equal(t, float64(100), *myTypeValue.Properties["age"].Value.Max)
}

func TestEmbeddedStructHandling(t *testing.T) {
	s := NewServer()

	// Define a struct with an embedded struct
	type InnerStruct struct {
		FieldA string `json:"field_a" example:"Value A" description:"A field in the inner struct"`
	}

	type OuterStruct struct {
		InnerStruct
		FieldB int `json:"field_b" example:"100" description:"B field in the outer struct"`
	}

	// Register a route that returns OuterStruct
	Get(s, "/embedded", func(ContextNoBody) (OuterStruct, error) {
		return OuterStruct{}, nil
	})

	// Generate OpenAPI spec
	document := s.OutputOpenAPISpec()
	require.NotNil(t, document)

	// Ensure that both the embedded and non-embedded fields are present in the schema
	outerSchema := document.Components.Schemas["OuterStruct"].Value
	require.NotNil(t, outerSchema)

	// Check if embedded struct fields are included
	require.NotNil(t, outerSchema.Properties["field_a"])
	require.Equal(t, &openapi3.Types{"string"}, outerSchema.Properties["field_a"].Value.Type)
	require.Equal(t, "Value A", outerSchema.Properties["field_a"].Value.Example)
	require.Equal(t, "A field in the inner struct", outerSchema.Properties["field_a"].Value.Description)

	// Check if non-embedded struct fields are included
	require.NotNil(t, outerSchema.Properties["field_b"])
	require.Equal(t, &openapi3.Types{"integer"}, outerSchema.Properties["field_b"].Value.Type)
	require.Equal(t, 100, outerSchema.Properties["field_b"].Value.Example)
	require.Equal(t, "B field in the outer struct", outerSchema.Properties["field_b"].Value.Description)
}

func TestDeclareCustom200Response(t *testing.T) {
	// A custom option to add a custom response to the OpenAPI spec.
	// The route returns a PNG image.
	optionReturnsPNG := func(br *BaseRoute) {
		response := openapi3.NewResponse()
		response.WithDescription("Generated image")
		response.WithContent(openapi3.NewContentWithSchema(nil, []string{"image/png"}))
		br.Operation.AddResponse(200, response)
	}

	s := NewServer()

	GetStd(s, "/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("PNG image"))
	}, optionReturnsPNG)

	openAPIResponse := s.OpenAPI.Description().Paths.Find("/image").Get.Responses.Value("200")
	require.Nil(t, openAPIResponse.Value.Content.Get("application/json"))
	require.NotNil(t, openAPIResponse.Value.Content.Get("image/png"))
	require.Equal(t, "Generated image", *openAPIResponse.Value.Description)
}
