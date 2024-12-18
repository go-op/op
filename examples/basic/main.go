package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
)

type Received struct {
	Name string `json:"name" validate:"required"`
}

type MyResponse struct {
	Message       string `json:"message"`
	BestFramework string `json:"best"`
}

func main() {
	s := fuego.NewServer(
		fuego.WithAddr("localhost:8088"),
	)

	fuego.Use(s, cors.Default().Handler)
	fuego.Use(s, chiMiddleware.Compress(5, "text/html", "text/css"))

	// Fuego 🔥 handler with automatic OpenAPI generation, validation, (de)serialization and error handling
	fuego.Post(s, "/", func(c fuego.ContextWithBody[Received]) (MyResponse, error) {
		data, err := c.Body()
		if err != nil {
			return MyResponse{}, err
		}

		// read the request header test
		if c.Request().Header.Get("test") != "test" {
			return MyResponse{}, errors.New("test header not equal to 'test'")
		}

		c.Response().Header().Set("X-Hello", "World")

		return MyResponse{
			Message:       "Hello, " + data.Name,
			BestFramework: "Fuego!",
		}, nil
	},
		option.Description("Say hello to the world"),
		option.Header("test", "Just a test header"),
		option.Cookie("test", "A Cookie!"),
	)

	// Standard net/http handler with automatic OpenAPI route declaration
	fuego.GetStd(s, "/std", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	s.Run()
}

// InTransform will be called when using c.Body().
// It can be used to transform the entity and raise custom errors
func (r *Received) InTransform(context.Context) error {
	r.Name = strings.ToLower(r.Name)
	if r.Name == "fuego" {
		return errors.New("fuego is not a name")
	}
	return nil
}

// OutTransform will be called before sending data
func (r *MyResponse) OutTransform(context.Context) error {
	r.Message = strings.ToUpper(r.Message)
	return nil
}

var (
	_ fuego.InTransformer  = &Received{}   // Ensure that *Received implements fuego.InTransformer
	_ fuego.OutTransformer = &MyResponse{} // Ensure that *MyResponse implements fuego.OutTransformer
)
