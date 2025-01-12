package fuego_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
)

func TestParams(t *testing.T) {
	t.Run("All options", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", func(c fuego.ContextNoBody) (string, error) {
			name := c.QueryParam("name")
			age := c.QueryParamInt("age")
			isok := c.QueryParamBool("is_ok")

			return name + strconv.Itoa(age) + strconv.FormatBool(isok), nil
		},
			option.Query("name", "Name", fuego.ParamRequired(), fuego.ParamDefault("hey"), fuego.ParamExample("example1", "you")),
			option.QueryInt("age", "Age", fuego.ParamNullable(), fuego.ParamDefault(18), fuego.ParamExample("example1", 1)),
			option.QueryBool("is_ok", "Is OK?", fuego.ParamDefault(true), fuego.ParamExample("example1", true)),
		)

		require.NotNil(t, route)
		require.NotNil(t, route.Params)
		require.Len(t, route.Params, 4)
		require.Equal(t, "Name", route.Params["name"].Description)
		require.True(t, route.Params["name"].Required)
		require.Equal(t, "hey", route.Params["name"].Default)
		require.Equal(t, "you", route.Params["name"].Examples["example1"])
		require.Equal(t, "string", route.Params["name"].GoType)

		require.Equal(t, "Age", route.Params["age"].Description)
		require.True(t, route.Params["age"].Nullable)
		require.Equal(t, 18, route.Params["age"].Default)
		require.Equal(t, "integer", route.Params["age"].GoType)
	})
}
