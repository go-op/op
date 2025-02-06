package views

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/static"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa"
	"github.com/go-fuego/fuego/extra/markdown"
)

// Resource is the struct that holds useful sources of information available for the controllers.
type Resource struct {
	DosingQueries      DosingRepository
	RecipesQueries     RecipeRepository
	IngredientsQueries IngredientRepository
	MetaQueries        MetaRepository
	Security           fuego.Security
}

func (rs Resource) showRecipesStd(w http.ResponseWriter, r *http.Request) {
	recipes, err := rs.RecipesQueries.GetRecipes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fp := path.Join("templates", "recipes.html")
	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, recipes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (rs Resource) robots(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, static.StaticFiles, "robots.txt")
}

func (rs Resource) showIndex(c fuego.ContextNoBody) (fuego.Templ, error) {
	timeDBRequest := time.Now()

	recipes, err := rs.RecipesQueries.GetRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	fastRecipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			Valid: true,
		},
		MaxTime:     15,
		MaxCalories: 99999,
		Published:   true,
	})
	if err != nil {
		return nil, err
	}

	timeHealthyRecipes := time.Now()
	healthyRecipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			Valid: true,
		},
		MaxTime:     99999,
		MaxCalories: 500,
		Published:   true,
	})
	if err != nil {
		return nil, err
	}

	c.Response().Header().Add("Server-Timing", fuego.Timing{
		Name: "dbHealthyRecipes",
		Dur:  time.Since(timeHealthyRecipes),
		Desc: "controller > db > healthy recipes",
	}.String())

	popularRecipes, err := rs.RecipesQueries.GetRandomRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	c.Response().Header().Add("Server-Timing", fuego.Timing{
		Name: "dbRequest",
		Dur:  time.Since(timeDBRequest),
		Desc: "controller > db",
	}.String())

	return templa.Home(templa.HomeProps{
		Recipes:        recipes,
		PopularRecipes: popularRecipes,
		FastRecipes:    fastRecipes,
		HealthyRecipes: healthyRecipes,
	}), nil
}

func (rs Resource) listRecipes(c fuego.ContextNoBody) (*fuego.DataOrTemplate[[]store.Recipe], error) {
	recipes, err := rs.RecipesQueries.GetRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	return fuego.DataOrHTML(
		recipes,
		templa.SearchPage(templa.SearchProps{
			Recipes: recipes,
		})), nil
}

func (rs Resource) relatedRecipes(c fuego.ContextNoBody) (*fuego.DataOrTemplate[[]store.Recipe], error) {
	baseRecipeID := c.PathParam("id")

	recipes, err := rs.RecipesQueries.GetRandomRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	filteredRecipes := make([]store.Recipe, 0, len(recipes))
	for _, r := range recipes {
		if r.ID == baseRecipeID {
			continue
		}
		filteredRecipes = append(filteredRecipes, r)
	}

	return fuego.DataOrHTML(
		filteredRecipes,
		templa.RelatedRecipes(filteredRecipes),
	), nil
}

func (rs Resource) showSingleRecipes2(c fuego.ContextNoBody) (fuego.Templ, error) {
	id := c.PathParam("id")

	recipe, err := rs.RecipesQueries.GetRecipe(c.Context(), id)
	if err != nil {
		return nil, fmt.Errorf("error getting recipe %s: %w", id, err)
	}

	ingredients, err := rs.IngredientsQueries.GetIngredientsOfRecipe(c.Context(), id)
	if err != nil {
		slog.Error("Error getting ingredients of recipe", "error", err)
	}

	adminCookie, _ := c.Request().Cookie("admin")

	return templa.RecipePage(templa.RecipePageProps{
		Recipe:         recipe,
		Ingredients:    ingredients,
		RelatedRecipes: []store.Recipe{{}, {}, {}, {}, {}},
	}, templa.GeneralProps{
		IsAdmin: adminCookie != nil,
	}), nil
}

func (rs Resource) searchRecipes(c fuego.ContextNoBody) (fuego.Templ, error) {
	search := c.QueryParam("q")

	recipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			String: search,
			Valid:  true,
		},
		MaxTime:     99999,
		MaxCalories: 99999,
		Published:   true,
	})
	if err != nil {
		return nil, err
	}

	return templa.SearchPage(templa.SearchProps{
		Recipes: recipes,
		Search:  search,
	}), nil
}

func (rs Resource) fastRecipes(c fuego.ContextNoBody) (fuego.Templ, error) {
	recipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			Valid: true,
		},
		MaxTime:     15,
		MaxCalories: 99999,
		Published:   true,
	})
	if err != nil {
		return nil, err
	}

	return templa.SearchPage(templa.SearchProps{
		Recipes: recipes,
		Filters: templa.SearchFilters{
			MaxTime: 15,
		},
	}), nil
}

func (rs Resource) healthyRecipes(c fuego.ContextNoBody) (fuego.Templ, error) {
	recipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			String: "",
			Valid:  true,
		},
		MaxTime:     99999,
		MaxCalories: 500,
		Published:   true,
	})
	if err != nil {
		return nil, err
	}

	return templa.SearchPage(templa.SearchProps{
		Recipes: recipes,
		Filters: templa.SearchFilters{
			MaxCalories: 500,
		},
	}), nil
}

func (rs Resource) showRecipesList(c fuego.ContextNoBody) (fuego.CtxRenderer, error) {
	search := c.QueryParam("search")
	recipes, err := rs.RecipesQueries.SearchRecipes(c.Context(), store.SearchRecipesParams{
		Search: sql.NullString{
			String: search,
			Valid:  true,
		},
		Published: true,
	})
	if err != nil {
		return nil, err
	}

	return c.Render("partials/recipes-list.partial.html", recipes)
}

func (rs Resource) addRecipe(c fuego.ContextWithBody[store.CreateRecipeParams]) (fuego.CtxRenderer, error) {
	body, err := c.Body()
	if err != nil {
		return nil, err
	}

	body.ID = uuid.NewString()

	_, err = rs.RecipesQueries.CreateRecipe(c.Context(), body)
	if err != nil {
		return nil, err
	}

	recipes, err := rs.RecipesQueries.GetRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	return c.Render("pages/admin.page.html", fuego.H{
		"Recipes": recipes,
	})
}

func (rs Resource) RecipePage(c fuego.ContextNoBody) (fuego.CtxRenderer, error) {
	id := c.PathParam("id")

	recipe, err := rs.RecipesQueries.GetRecipe(c.Context(), id)
	if err != nil {
		return nil, fmt.Errorf("error getting recipe %s: %w", id, err)
	}

	ingredients, err := rs.IngredientsQueries.GetIngredientsOfRecipe(c.Context(), id)
	if err != nil {
		slog.Error("Error getting ingredients of recipe", "error", err)
	}

	return c.Render("pages/recipe.page.html", fuego.H{
		"Recipe":       recipe,
		"Ingredients":  ingredients,
		"Instructions": markdown.Markdown(recipe.Instructions),
	})
}

func (rs Resource) getAllRecipesStandardWithHelpers(w http.ResponseWriter, r *http.Request) {
	recipes, err := rs.RecipesQueries.GetRecipes(r.Context())
	if err != nil {
		fuego.SendJSONError(w, r, err)
		return
	}

	fuego.SendJSON(w, r, recipes)
}

type RecipeRepository interface {
	CreateRecipe(ctx context.Context, arg store.CreateRecipeParams) (store.Recipe, error)
	DeleteRecipe(ctx context.Context, id string) error
	GetRecipe(ctx context.Context, id string) (store.Recipe, error)
	UpdateRecipe(ctx context.Context, arg store.UpdateRecipeParams) (store.Recipe, error)
	GetRecipes(ctx context.Context) ([]store.Recipe, error)
	GetRandomRecipes(ctx context.Context) ([]store.Recipe, error)
	SearchRecipes(ctx context.Context, params store.SearchRecipesParams) ([]store.Recipe, error)
}

var _ RecipeRepository = (*store.Queries)(nil)
