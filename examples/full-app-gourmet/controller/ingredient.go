package controller

import (
	"context"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
)

type ingredientResource struct {
	IngredientRepository IngredientRepository
}

func (rs ingredientResource) MountRoutes(s *fuego.Server) {
	ingredientsGroup := fuego.Group(s, "/ingredients")
	fuego.Get(ingredientsGroup, "/ingredients", rs.getAllIngredients)
	fuego.Post(ingredientsGroup, "/ingredients/new", rs.newIngredient)
}

func (rs ingredientResource) getAllIngredients(c fuego.ContextNoBody) ([]store.Ingredient, error) {
	ingredients, err := rs.IngredientRepository.GetIngredients(c.Context())
	if err != nil {
		return nil, err
	}

	return ingredients, nil
}

type CreateIngredient struct {
	Name        string `json:"name" validate:"required,min=3,max=20"`
	Description string `json:"description"`
}

func (ci *CreateIngredient) InTransform(context.Context) error {
	if ci.Description == "" {
		ci.Description = "No description"
	}
	ci.Name = strings.TrimSpace(ci.Name)
	return nil
}

func (rs ingredientResource) newIngredient(c fuego.ContextWithBody[CreateIngredient]) (store.Ingredient, error) {
	body, err := c.Body()
	if err != nil {
		return store.Ingredient{}, err
	}

	payload := store.CreateIngredientParams{
		ID:          generateID(),
		Name:        body.Name,
		Description: body.Description,
	}

	ingredient, err := rs.IngredientRepository.CreateIngredient(c.Context(), payload)
	if err != nil {
		return store.Ingredient{}, err
	}

	return ingredient, nil
}

type IngredientRepository interface {
	CreateIngredient(ctx context.Context, arg store.CreateIngredientParams) (store.Ingredient, error)
	GetIngredient(ctx context.Context, id string) (store.Ingredient, error)
	GetIngredients(ctx context.Context) ([]store.Ingredient, error)
	GetIngredientsOfRecipe(ctx context.Context, recipeID string) ([]store.GetIngredientsOfRecipeRow, error)
}

var _ IngredientRepository = (*store.Queries)(nil)
