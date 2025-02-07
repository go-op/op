package handler

import (
	"context"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa"
)

func (rs Resource) showIngredients(c fuego.ContextNoBody) (fuego.Templ, error) {
	ingredients, _ := rs.IngredientsQueries.GetIngredients(c.Context())

	if c.Header("HX-Request") == "true" && c.Header("HX-Target") == "#page" {
		return templa.IngredientList(templa.IngredientListProps{
			Ingredients: ingredients,
		}), nil
	}

	// headerInfo, _ := rs.MetaQueries.GetHeaderInfo(c.Context())

	return templa.IngredientPage(templa.IngredientPageProps{
		Ingredients: ingredients,
		// Header:      headerInfo,
	}), nil
}

type IngredientRepository interface {
	CreateIngredient(ctx context.Context, arg store.CreateIngredientParams) (store.Ingredient, error)
	GetIngredient(ctx context.Context, id string) (store.Ingredient, error)
	GetIngredients(ctx context.Context) ([]store.Ingredient, error)
	GetIngredientsOfRecipe(ctx context.Context, recipeID string) ([]store.GetIngredientsOfRecipeRow, error)
	UpdateIngredient(ctx context.Context, arg store.UpdateIngredientParams) (store.Ingredient, error)
	SearchIngredients(ctx context.Context, arg store.SearchIngredientsParams) ([]store.Ingredient, error)
}

type MetaRepository interface {
	GetHeaderInfo(ctx context.Context) (string, error)
}

var _ IngredientRepository = (*store.Queries)(nil)
