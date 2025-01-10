package fuego_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"

	"github.com/go-fuego/fuego"
)

type GenericInput[T any] struct {
	Thing string `json:"thing"`
	Data  T      `json:"data"`
}

type GenericResponse[Res any] struct {
	StatusCode int    `json:"statusCode"`
	Result     Res    `json:"result"`
	Message    string `json:"message"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name" validate:"required,min=1,max=100" example:"Napoleon"`
}

func TestGenericReturnType(t *testing.T) {
	s := fuego.NewServer()
	route := fuego.Get(s, "/test", func(c fuego.ContextWithBody[GenericInput[User]]) (*GenericResponse[User], error) {
		body, err := c.Body()
		if err != nil {
			return nil, err
		}

		return &GenericResponse[User]{
			StatusCode: 200,
			Result:     User{ID: 1, Name: body.Data.Name},
			Message:    "success",
		}, nil
	})

	// Request OpenAPI
	t.Log(route.Operation.RequestBody)
	require.NotNil(t, route.Operation.RequestBody.Value.Content["*/*"])
	requestType := route.Operation.RequestBody.Value.Content["*/*"].Schema.Value
	require.Equal(t, &openapi3.Types{"object"}, requestType.Type)
	require.Equal(t, &openapi3.Types{"string"}, requestType.Properties["thing"].Value.Type)
	require.Equal(t, &openapi3.Types{"object"}, requestType.Properties["data"].Value.Type)
	require.Equal(t, &openapi3.Types{"integer"}, requestType.Properties["data"].Value.Properties["id"].Value.Type)

	// Response OpenAPI
	responseType := route.Operation.Responses.Value("200").Value.Content["application/json"].Schema.Value
	require.Equal(t, &openapi3.Types{"integer"}, responseType.Properties["statusCode"].Value.Type)

	resultResponseType := responseType.Properties["result"].Value
	require.Equal(t, &openapi3.Types{"object"}, resultResponseType.Type)
	require.Equal(t, &openapi3.Types{"integer"}, resultResponseType.Properties["id"].Value.Type)
	require.Equal(t, &openapi3.Types{"string"}, resultResponseType.Properties["name"].Value.Type)

	// Behavior at runtime

	t.Run("Happy path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", strings.NewReader(`{"thing":"user","data":{"id":1,"name":"Napoleon"}}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()

		s.Mux.ServeHTTP(res, req)

		require.Equal(t, 200, res.Code)
		response := res.Body.String()
		require.JSONEq(t, `{"statusCode":200,"result":{"id":1,"name":"Napoleon"},"message":"success"}`, response)
	})

	t.Run("Generic request still support nested validation tags", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()

		s.Mux.ServeHTTP(res, req)

		require.Equal(t, 400, res.Code)
		response := res.Body.String()
		require.JSONEq(t, `{"title":"Validation Error","detail":"Name is required","errors":[{"more":{"field":"Name","nsField":"GenericInput[github.com/go-fuego/fuego_test.User].Data.Name","param":"","tag":"required","value":""},"name":"GenericInput[github.com/go-fuego/fuego_test.User].Data.Name","reason":"Key: 'GenericInput[github.com/go-fuego/fuego_test.User].Data.Name' Error:Field validation for 'Name' failed on the 'required' tag"}],"status":400}`, response)
	})
}
