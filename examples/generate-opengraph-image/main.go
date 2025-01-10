package main

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/generate-opengraph-image/controller"
	"github.com/go-fuego/fuego/middleware/cache"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

// A custom option to add a custom response to the OpenAPI spec.
// The route returns a PNG image.
var optionReturnsPNG = func(br *fuego.BaseRoute) {
	response := openapi3.NewResponse()
	response.WithDescription("Generated image")
	response.WithContent(openapi3.NewContentWithSchema(nil, []string{"image/png"}))
	br.Operation.AddResponse(200, response)
}

func main() {
	s := fuego.NewServer(
		fuego.WithEngineOptions(
			fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
				PrettyFormatJSON: true,
			}),
		),
	)

	fuego.GetStd(s, "/{title}", controller.OpenGraphHandler,
		optionReturnsPNG,
		option.Description("Generate an image with a title. Useful for Opengraph."),
		option.Path("title", "The title to write on the image", param.Example("example", "My awesome article!")),
		option.Middleware(cache.New()),
	)

	s.Run()
}
