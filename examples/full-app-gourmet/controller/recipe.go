package controller

import (
	"context"
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
	"github.com/go-fuego/fuego/option"
)

type recipeResource struct {
	RecipeRepository     RecipeRepository
	IngredientRepository IngredientRepository
}

func (rs recipeResource) MountRoutes(s *fuego.Server) {
	fuego.GetStd(s, "/recipes-standard-with-helpers", rs.getAllRecipesStandardWithHelpers,
		option.Tags("Recipe"),
	)
	recipeGroup := fuego.Group(s, "/recipes")

	fuego.Get(recipeGroup, "/", rs.getAllRecipes,
		option.Summary("Get all recipes"),
		option.Description("Get all recipes"),
		option.Query("limit", "number of recipes to return"),
		option.Tags("customer"),
	)
	fuego.Post(recipeGroup, "/new", rs.newRecipe)
	fuego.Get(recipeGroup, "/{id}", rs.getRecipeWithIngredients)
}

func (rs recipeResource) getAllRecipesStandardWithHelpers(w http.ResponseWriter, r *http.Request) {
	recipes, err := rs.RecipeRepository.GetRecipes(r.Context())
	if err != nil {
		fuego.SendJSONError(w, r, err)
		return
	}

	fuego.SendJSON(w, r, recipes)
}

func (rs recipeResource) getAllRecipes(c fuego.ContextNoBody) ([]store.Recipe, error) {
	recipes, err := rs.RecipeRepository.GetRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	return recipes, nil
}

func (rs recipeResource) newRecipe(c fuego.ContextWithBody[store.CreateRecipeParams]) (store.Recipe, error) {
	body, err := c.Body()
	if err != nil {
		return store.Recipe{}, err
	}

	recipe, err := rs.RecipeRepository.CreateRecipe(c.Context(), body)
	if err != nil {
		return store.Recipe{}, err
	}

	return recipe, nil
}

func (rs recipeResource) getRecipeWithIngredients(c fuego.ContextNoBody) ([]store.GetIngredientsOfRecipeRow, error) {
	recipe, err := rs.IngredientRepository.GetIngredientsOfRecipe(c.Context(), c.PathParam("id"))
	if err != nil {
		return nil, err
	}

	return recipe, nil
}

type RecipeRepository interface {
	CreateRecipe(ctx context.Context, arg store.CreateRecipeParams) (store.Recipe, error)
	DeleteRecipe(ctx context.Context, id string) error
	GetRecipe(ctx context.Context, id string) (store.Recipe, error)
	GetRecipes(ctx context.Context) ([]store.Recipe, error)
	SearchRecipes(ctx context.Context, args store.SearchRecipesParams) ([]store.Recipe, error)
}

var _ RecipeRepository = (*store.Queries)(nil)
