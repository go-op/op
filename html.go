package fuego

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

// CtxRenderer is an interface that can be used to render a response.
// It is used with standard library templating engine, by using fuego.ContextXXX.Render
// It is compatible with [github.com/a-h/templ] out of the box.
// Example:
//
//	func getRecipes(ctx fuego.ContextNoBody) (fuego.CtxRenderer, error) {
//		recipes, err := ctx.store.GetRecipes(ctx.Context())
//		if err != nil {
//			return nil, err
//		}
//
//		return recipeComponent(recipes), nil // recipeComponent is templ component
//	}
type CtxRenderer interface {
	Render(context.Context, io.Writer) error
}

// Templ is a shortcut for [CtxRenderer], which can be used with [github.com/a-h/templ]
type Templ = CtxRenderer

// Renderer can be used with [github.com/maragudk/gomponents]
// Example:
//
//	func getRecipes(ctx fuego.ContextNoBody) (fuego.CtxRenderer, error) {
//		recipes, err := ctx.store.GetRecipes(ctx.Context())
//		if err != nil {
//			return nil, err
//		}
//
//		return recipeComponent(recipes), nil // recipeComponent is gomponents component
//	}
type Renderer interface {
	Render(io.Writer) error
}

// Gomponent is a shortcut for [Renderer], which can be used with [github.com/maragudk/gomponents]
type Gomponent = Renderer

// HTML is a marker type used to differentiate between a string response and an HTML response.
// To use templating, use [Context.Render].
type HTML string

// H is a shortcut for map[string]any
type H map[string]any

// StdRenderer renders a template using the standard library templating engine.
type StdRenderer struct {
	fs                fs.FS
	data              any
	templates         *template.Template
	templateToExecute string
	layoutsGlobs      []string
}

var _ CtxRenderer = StdRenderer{}

func (s StdRenderer) Render(ctx context.Context, w io.Writer) error {
	if strings.Contains(s.templateToExecute, "/") || strings.Contains(s.templateToExecute, "*") {
		s.layoutsGlobs = append(s.layoutsGlobs, s.templateToExecute) // To override all blocks defined in the main template
		cloned := template.Must(s.templates.Clone())
		tmpl, err := cloned.ParseFS(s.fs, s.layoutsGlobs...)
		if err != nil {
			return HTTPError{
				Err:    err,
				Status: http.StatusInternalServerError,
				Title:  "Error parsing template",
				Detail: fmt.Errorf("error parsing template '%s': %w", s.layoutsGlobs, err).Error(),
				Errors: []ErrorItem{
					{
						Name:   "templates",
						Reason: "Check that the template exists and have the correct extension. Globs: " + strings.Join(s.layoutsGlobs, ", "),
					},
				},
			}
		}
		s.templates = template.Must(tmpl.Clone())
	}

	// Get only last template name (for example, with partials/nav/main/nav.partial.html, get nav.partial.html)
	myTemplate := strings.Split(s.templateToExecute, "/")
	s.templateToExecute = myTemplate[len(myTemplate)-1]

	err := s.templates.ExecuteTemplate(w, s.templateToExecute, s.data)
	if err != nil {
		return HTTPError{
			Err:    err,
			Status: http.StatusInternalServerError,
			Title:  "Error rendering template",
			Detail: fmt.Errorf("error executing template '%s': %w", s.templateToExecute, err).Error(),
			Errors: []ErrorItem{
				{
					Name:   "templates",
					Reason: "Check that the template exists and have the correct extension. Template: " + s.templateToExecute,
				},
			},
		}
	}

	return err
}

// loadTemplates
func (s *Server) loadTemplates(patterns ...string) error {
	tmpl, err := template.ParseFS(s.fs, patterns...)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	s.template = tmpl

	return nil
}
