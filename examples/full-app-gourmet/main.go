package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/joho/godotenv"
	"github.com/lithammer/shortuuid/v4"
	"github.com/lmittmann/tint"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/handler"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/server"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
)

//go:generate sqlc generate

func main() {
	// Load .env.local then .env files
	err := godotenv.Load(".env.local", ".env")
	if err != nil {
		wd, _ := os.Getwd()
		slog.Error(fmt.Sprintf("Error loading .env files: %s in dir %s", err, wd))
	}

	// Flags
	port := flag.Int("port", 8083, "port to listen to")
	dbPath := flag.String("db", "./recipe.db", "path to database file")
	debug := flag.Bool("debug", false, "debug mode")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	// Set my custom colored logger
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			AddSource:  *debug,
			Level:      logLevel,
			TimeFormat: "15:04:05",
		}),
	))

	// Connect to database
	db := store.InitDB(*dbPath)

	store := store.New(db)

	// Create resources that will be available in handlers
	viewsResources := handler.Resource{
		RecipesQueries:     store,
		IngredientsQueries: store,
		DosingQueries:      store,
		UsersQueries:       store,
		FavoritesQueries:   store,
		Security:           fuego.NewSecurity(),
	}

	rs := server.Resources{
		HandlersResources: viewsResources,
		CorsOrigins:       strings.Split(os.Getenv("CORS_ORIGINS"), ","),
	}

	app := rs.Setup(
		fuego.WithAddr(fmt.Sprintf(":%d", *port)),
		fuego.WithLoggingMiddleware(fuego.LoggingConfig{
			RequestIDFunc: func() string { return shortuuid.New() },
		}),
	)

	app.OpenAPI.Description().Servers = append(app.OpenAPI.Description().Servers, &openapi3.Server{
		URL:         os.Getenv("PUBLIC_URL"),
		Description: "Production server",
	})

	// Run the server!
	err = app.Run()
	if err != nil {
		slog.Error("Error running server: %s", "err", err)
	}
}
