package fuego

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// NewEngine creates a new Engine with the given options.
// For example:
//
//	engine := fuego.NewEngin(
//		WithOpenAPIConfig(
//			OpenAPIConfig{
//				PrettyFormatJSON: true,
//			},
//		),
//	)
//
// Options all begin with `With`.
func NewEngine(options ...func(*Engine)) *Engine {
	e := &Engine{
		OpenAPI:       NewOpenAPI(),
		OpenAPIConfig: defaultOpenAPIConfig,
		ErrorHandler:  ErrorHandler,
	}
	for _, option := range options {
		option(e)
	}
	return e
}

// The Engine is the main struct of the framework.
type Engine struct {
	OpenAPI       *OpenAPI
	ErrorHandler  func(error) error
	OpenAPIConfig OpenAPIConfig

	requestContentTypes []string
}

type OpenAPIConfig struct {
	// Local path to save the OpenAPI JSON spec
	JSONFilePath string
	// If true, the server will not serve nor generate any OpenAPI resources
	Disabled bool
	// If true, the engine will not print messages
	DisableMessages bool
	// If true, the engine will not save the OpenAPI JSON spec locally
	DisableLocalSave bool
	// Pretty prints the OpenAPI spec with proper JSON indentation
	PrettyFormatJSON bool
}

var defaultOpenAPIConfig = OpenAPIConfig{
	JSONFilePath: "doc/openapi.json",
}

// WithRequestContentType sets the accepted content types for the engine.
// By default, the accepted content types is */*.
func WithRequestContentType(consumes ...string) func(*Engine) {
	return func(e *Engine) { e.requestContentTypes = consumes }
}

func WithOpenAPIConfig(config OpenAPIConfig) func(*Engine) {
	return func(e *Engine) {
		if config.JSONFilePath != "" {
			e.OpenAPIConfig.JSONFilePath = config.JSONFilePath
		}

		e.OpenAPIConfig.Disabled = config.Disabled
		e.OpenAPIConfig.DisableLocalSave = config.DisableLocalSave
		e.OpenAPIConfig.PrettyFormatJSON = config.PrettyFormatJSON
	}
}

// WithErrorHandler sets a customer error handler for the server
func WithErrorHandler(errorHandler func(err error) error) func(*Engine) {
	return func(e *Engine) {
		if errorHandler == nil {
			panic("errorHandler cannot be nil")
		}

		e.ErrorHandler = errorHandler
	}
}

// DisableErrorHandler overrides ErrorHandler with a simple pass-through
func DisableErrorHandler() func(*Engine) {
	return func(e *Engine) {
		e.ErrorHandler = func(err error) error { return err }
	}
}

// OutputOpenAPISpec takes the OpenAPI spec and outputs it to a JSON file
func (e *Engine) OutputOpenAPISpec() []byte {
	e.OpenAPI.computeTags()

	// Validate
	err := e.OpenAPI.Description().Validate(context.Background())
	if err != nil {
		slog.Error("Error validating spec", "error", err)
	}

	// Marshal spec to JSON
	jsonSpec, err := e.marshalSpec()
	if err != nil {
		slog.Error("Error marshaling spec to JSON", "error", err)
	}

	if !e.OpenAPIConfig.DisableLocalSave {
		err := e.saveOpenAPIToFile(e.OpenAPIConfig.JSONFilePath, jsonSpec)
		if err != nil {
			slog.Error("Error saving spec to local path", "error", err, "path", e.OpenAPIConfig.JSONFilePath)
		}
	}
	return jsonSpec
}

func (e *Engine) saveOpenAPIToFile(jsonSpecLocalPath string, jsonSpec []byte) error {
	jsonFolder := filepath.Dir(jsonSpecLocalPath)

	err := os.MkdirAll(jsonFolder, 0o750)
	if err != nil {
		return fmt.Errorf("error creating docs directory: %w", err)
	}

	f, err := os.Create(jsonSpecLocalPath) // #nosec G304 (file path provided by developer, not by user)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(jsonSpec)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	e.printOpenAPIMessage("JSON file: " + jsonSpecLocalPath)
	return nil
}

func (s *Engine) marshalSpec() ([]byte, error) {
	if s.OpenAPIConfig.PrettyFormatJSON {
		return json.MarshalIndent(s.OpenAPI.Description(), "", "\t")
	}
	return json.Marshal(s.OpenAPI.Description())
}

func (e *Engine) printOpenAPIMessage(msg string) {
	if !e.OpenAPIConfig.DisableMessages {
		slog.Info(msg)
	}
}
