package fuego

import (
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"time"
)

// Run starts the server.
// It is blocking.
// It returns an error if the server could not start (it could not bind to the port for example).
// It also generates the OpenAPI spec and outputs it to a file, the UI, and a handler (if enabled).
func (s *Server) Run() error {
	if err := s.setup(); err != nil {
		return err
	}
	return s.Server.Serve(s.listener)
}

// RunTLS starts the server with a TLS listener
// It is blocking.
// It returns an error if the server could not start (it could not bind to the port for example).
// It also generates the OpenAPI spec and outputs it to a file, the UI, and a handler (if enabled).
func (s *Server) RunTLS(certFile, keyFile string) error {
	s.isTLS = true
	if err := s.setup(); err != nil {
		return err
	}
	return s.Server.ServeTLS(s.listener, certFile, keyFile)
}

func (s *Server) setup() error {
	if err := s.setupDefaultListener(); err != nil {
		return err
	}
	go s.OutputOpenAPISpec()
	RegisterOpenAPIRoutes(s.Engine, s)
	s.printStartupMessage()

	s.Server.Handler = s.Mux
	if s.corsMiddleware != nil {
		s.Server.Handler = s.corsMiddleware(s.Server.Handler)
	}

	return nil
}

func (s *Server) setupDefaultListener() error {
	if s.listener != nil {
		s.Addr = s.listener.Addr().String()
		return nil
	}
	listener, err := net.Listen("tcp", s.Addr)
	s.listener = listener
	return err
}

func (s *Server) printStartupMessage() {
	if !s.disableStartupMessages {
		elapsed := time.Since(s.startTime)
		slog.Debug("Server started in "+elapsed.String(), "info", "time between since server creation (fuego.NewServer) and server startup (fuego.Run). Depending on your implementation, there might be things that do not depend on fuego slowing start time")
		slog.Info("Server running ✅ on "+s.URL(), "started in", elapsed.String())
	}
}

func (s *Server) proto() string {
	if s.isTLS {
		return "https"
	}
	return "http"
}

func (s *Server) URL() string {
	return s.proto() + "://" + s.Server.Addr
}

// HTTPHandler converts a Fuego controller into a http.HandlerFunc.
// Uses Server for configuration.
// Uses Route for route configuration. Optional.
func HTTPHandler[ReturnType, Body any](s *Server, controller func(c ContextWithBody[Body]) (ReturnType, error), route BaseRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var templates *template.Template
		if s.template != nil {
			templates = template.Must(s.template.Clone())
		}

		// CONTEXT INITIALIZATION
		ctx := NewNetHTTPContext[Body](route, w, r, readOptions{
			DisallowUnknownFields: s.DisallowUnknownFields,
			MaxBodySize:           s.maxBodySize,
		})
		ctx.serializer = s.Serialize
		ctx.errorSerializer = s.SerializeError
		ctx.fs = s.fs
		ctx.templates = templates

		Flow(s.Engine, ctx, controller)
	}
}

// Contains the logic for the flow of a Fuego controller.
// Extends ContextWithBody with methods not exposed in the Controllers.
type ContextFlowable[B any] interface {
	ContextWithBody[B]

	// SetDefaultStatusCode sets the status code of the response defined in the options.
	SetDefaultStatusCode()
	// Serialize serializes the given data to the response.
	Serialize(data any) error
	// SerializeError serializes the given error to the response.
	SerializeError(err error)
}

// Generic handler for Fuego controllers.
func Flow[B, T any](s *Engine, ctx ContextFlowable[B], controller func(c ContextWithBody[B]) (T, error)) {
	ctx.SetHeader("X-Powered-By", "Fuego")
	ctx.SetHeader("Trailer", "Server-Timing")

	timeCtxInit := time.Now()

	// PARAMS VALIDATION
	err := ValidateParams(ctx)
	if err != nil {
		err = s.ErrorHandler(err)
		ctx.SerializeError(err)
		return
	}

	timeController := time.Now()
	ctx.SetHeader("Server-Timing", Timing{"fuegoReqInit", "", timeController.Sub(timeCtxInit)}.String())

	// CONTROLLER
	ans, err := controller(ctx)
	if err != nil {
		err = s.ErrorHandler(err)
		ctx.SerializeError(err)
		return
	}
	ctx.SetHeader("Server-Timing", Timing{"controller", "", time.Since(timeController)}.String())

	ctx.SetDefaultStatusCode()

	if reflect.TypeOf(ans) == nil {
		return
	}

	// TRANSFORM OUT
	timeTransformOut := time.Now()
	ans, err = transformOut(ctx.Context(), ans)
	if err != nil {
		err = s.ErrorHandler(err)
		ctx.SerializeError(err)
		return
	}
	timeAfterTransformOut := time.Now()
	ctx.SetHeader("Server-Timing", Timing{"transformOut", "transformOut", timeAfterTransformOut.Sub(timeTransformOut)}.String())

	// SERIALIZATION
	err = ctx.Serialize(ans)
	if err != nil {
		err = s.ErrorHandler(err)
		ctx.SerializeError(err)
	}
	ctx.SetHeader("Server-Timing", Timing{"serialize", "", time.Since(timeAfterTransformOut)}.String())
}
