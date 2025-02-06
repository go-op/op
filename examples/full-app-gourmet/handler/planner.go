package handler

import (
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa"
)

func (rs Resource) planner(c fuego.ContextNoBody) (fuego.Templ, error) {
	recipes, err := rs.RecipesQueries.GetRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	fastRecipes, err := rs.RecipesQueries.GetRandomRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	healthyRecipes, err := rs.RecipesQueries.GetRandomRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	popularRecipes, err := rs.RecipesQueries.GetRandomRecipes(c.Context())
	if err != nil {
		return nil, err
	}

	return templa.Planner(templa.PlannerProps{
		Recipes:        recipes,
		PopularRecipes: popularRecipes,
		FastRecipes:    fastRecipes,
		HealthyRecipes: healthyRecipes,
	}), nil
}
