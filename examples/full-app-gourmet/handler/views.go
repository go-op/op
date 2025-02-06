package handler

import (
	"net/http"
	"os"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/middleware/basicauth"
	"github.com/go-fuego/fuego/middleware/cache"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

var optionPagination = option.Group(
	option.QueryInt("page", "Page number", param.Default(1), param.Example("1st page", 1), param.Example("42nd page", 42)),
	option.QueryInt("perPage", "Number of items per page", param.Default(20)),
)

func (rs Resource) Routes(s *fuego.Server) {
	// Public Pages
	fuego.GetStd(s, "/recipes-std", rs.showRecipesStd, option.Tags("tests"))
	fuego.GetStd(s, "/recipes-std-json", rs.getAllRecipesStandardWithHelpers, option.Tags("tests"))
	fuego.All(s, "/", rs.showIndex, option.Middleware(cache.New()))
	fuego.GetStd(s, "/robots.txt", rs.robots, option.Middleware(cache.New()))
	fuego.Get(s, "/recipes", rs.listRecipes, option.Tags("recipes"))
	fuego.Get(s, "/planner", rs.planner)
	fuego.Get(s, "/recipes/{id}", rs.showSingleRecipes2, option.Tags("recipes"))
	fuego.Get(s, "/recipes/{id}/related", rs.relatedRecipes, option.Tags("recipes"))
	fuego.Post(s, "/recipes-new", rs.addRecipe, option.Tags("recipes"))
	fuego.Get(s, "/ingredients", rs.showIngredients, option.Tags("ingredients"))
	fuego.Get(s, "/fast", rs.fastRecipes, option.Tags("recipes"))
	fuego.Get(s, "/healthy", rs.healthyRecipes, option.Tags("recipes"))

	// Public Chunks
	fuego.Get(s, "/recipes-list", rs.showRecipesList,
		option.Query("search", "Search query", param.Example("example", "Galette des Rois")),
		option.Deprecated(),
		option.Tags("tests"),
	)
	fuego.Get(s, "/search", rs.searchRecipes,
		option.Query("q", "Search query", param.Required(), param.Example("example", "Galette des Rois")),
		option.AddError(http.StatusUnauthorized, "Authorization Error"),
		option.AddError(500, "My Server Error"),
		option.Tags("recipes"),
	)
	fuego.Get(s, "/ingredients/preselect-unit", rs.unitPreselected,
		option.Query("id", "ID", param.Required(), param.Default("1"), param.Example("example", "abcde1245")),
		option.Tags("ingredients"),
	)

	// Admin Pages
	adminRoutes := fuego.Group(s, "/admin",
		option.Middleware(basicauth.New(basicauth.Config{Username: os.Getenv("ADMIN_USER"), Password: os.Getenv("ADMIN_PASSWORD")})),
	)

	fuego.Get(adminRoutes, "", rs.pageAdmin,
		optionPagination,
	)
	fuego.Get(adminRoutes, "/recipes", rs.adminRecipes,
		optionPagination,
		option.Query("name", "Name to perform LIKE search on"),
	)
	fuego.Get(adminRoutes, "/recipes/{id}", rs.adminOneRecipe)
	fuego.Put(adminRoutes, "/recipes/{id}", rs.adminOneRecipe)
	fuego.Delete(adminRoutes, "/recipes/{id}", rs.adminDeleteRecipe)
	fuego.Get(adminRoutes, "/recipes/create", rs.adminCreateRecipePage)
	fuego.Put(adminRoutes, "/recipes/edit", rs.editRecipe)
	fuego.Post(adminRoutes, "/recipes/new", rs.adminAddRecipes)
	fuego.Post(adminRoutes, "/dosings/new", rs.adminAddDosing)
	fuego.Get(adminRoutes, "/ingredients", rs.adminIngredients,
		optionPagination,
		option.Query("name", "Name to perform LIKE search on"),
	)
	fuego.Get(adminRoutes, "/ingredients/create", rs.adminIngredientCreationPage)
	fuego.All(adminRoutes, "/ingredients/{id}", rs.adminOneIngredient)

	fuego.Post(adminRoutes, "/ingredients/new", rs.adminCreateIngredient,
		option.Description("Create a new ingredient"),
	)
	fuego.Get(adminRoutes, "/users", rs.adminRecipes)
}
