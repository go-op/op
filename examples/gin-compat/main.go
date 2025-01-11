package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/extra/fuegogin"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

type HelloRequest struct {
	Word string `json:"word" validate:"required,min=2"`
}

var _ fuego.InTransformer = &HelloRequest{}

type HelloResponse struct {
	Message string `json:"message"`
}

func main() {
	a := server()

	fmt.Println("OpenAPI at at http://localhost:8980/swagger ✅")

	err := a.Gin.Run(":8980")
	if err != nil {
		panic(err)
	}
}

func server() *fuegogin.Adaptor {
	ginRouter := gin.Default()
	engine := fuego.NewEngine()

	// Register Gin controller
	ginRouter.GET("/gin", ginController)

	// Incrementally add OpenAPI spec
	// 1️⃣ Level 1: Register Gin controller to Gin router, plugs Fuego OpenAPI route declaration
	fuegogin.GetGin(engine, ginRouter, "/gin-with-openapi", ginController)

	// 2️⃣ Level 2: Register Gin controller to Gin router, manually add options (not checked inside the Gin controller)
	fuegogin.GetGin(engine, ginRouter, "/gin-with-openapi-and-options", ginController,
		// OpenAPI options
		option.Summary("Gin controller with options"),
		option.Description("Some description"),
		option.OperationID("MyCustomOperationID"),
		option.Tags("Gin"),
	)

	// 3️⃣ Level 3: Register Fuego controller to Gin router. Fuego take care of serialization/deserialization, error handling, content-negotiation, etc.
	fuegogin.Get(engine, ginRouter, "/fuego", fuegoControllerGet)

	// 4️⃣ Level 4: Add some options to the POST endpoint (checks at start-time + validations at request time)
	fuegogin.Post(engine, ginRouter, "/fuego-with-options", fuegoControllerPost,
		// OpenAPI options
		option.Description("Some description"),
		option.OperationID("SomeOperationID"),
		option.AddError(409, "Name Already Exists"),
		option.DefaultStatusCode(201),
		option.Tags("Fuego"),

		// Add some parameters.
		option.Query("name", "Your name", param.Example("name example", "John Carmack")),
		option.Header("X-Request-ID", "Request ID", param.Default("123456")),
		option.Header("Content-Type", "Content Type", param.Default("application/json")),
	)

	// Supports groups & path parameters even for gin handlers
	group := ginRouter.Group("/my-group/:id")
	fuegogin.Get(engine, group, "/fuego", fuegoControllerGet,
		option.Summary("Route with group and id"),
		option.Tags("Fuego"),
	)

	// Serve the OpenAPI spec
	a := &fuegogin.Adaptor{Engine: engine, Gin: ginRouter}
	fuego.RegisterOpenAPIRoutes(a.Engine, a)

	return a
}

func (h *HelloRequest) InTransform(ctx context.Context) error {
	// Transformation
	h.Word = strings.ToLower(h.Word)

	// Custom validation, with fuego provided error
	if h.Word == "apple" {
		return fuego.BadRequestError{Title: "Word not allowed", Err: errors.New("forbidden word"), Detail: "The word 'apple' is not allowed"}
	}

	// Custom validation, with basic error
	if h.Word == "banana" {
		return errors.New("banana is not allowed")
	}

	// Context-based transformation
	if user := ctx.Value("user"); user == "secret agent" {
		h.Word = "*****"
	}

	return nil
}
