package controller

import (
	"context"
	"strings"

	"simple-crud/store"

	"github.com/go-fuego/fuego"
)

type ingredientRessource struct {
	IngredientRepository IngredientRepository
}

func (rs ingredientRessource) MountRoutes(s *fuego.Server) {
	fuego.Get(s, "/ingredients", rs.getAllIngredients)
	fuego.Post(s, "/ingredients/new", rs.newIngredient)
}

func (rs ingredientRessource) getAllIngredients(c fuego.Ctx[any]) ([]store.Ingredient, error) {
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

func (rs ingredientRessource) newIngredient(c fuego.Ctx[CreateIngredient]) (store.Ingredient, error) {
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
