package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/petstore/models"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

// default pagination options
var optionPagination = option.Group(
	option.QueryInt("per_page", "Number of items per page", param.Required()),
	option.QueryInt("page", "Page number", param.Default(1), param.Example("1st page", 1), param.Example("42nd page", 42), param.Example("100th page", 100)),
	option.ResponseHeader("Content-Range", "Total number of pets", param.StatusCodes(200, 206), param.Example("42 pets", "0-10/42")),
)

type PetsResources struct {
	PetsService PetsService
}

type PetsError struct {
	Err     error  `json:"-" xml:"-"`
	Message string `json:"message" xml:"message"`
}

var _ error = PetsError{}

func (e PetsError) Error() string { return e.Err.Error() }

func (rs PetsResources) Routes(s *fuego.Server) {
	petsGroup := fuego.Group(s, "/pets", option.Header("X-Header", "header description"))

	fuego.Get(petsGroup, "/", rs.filterPets,
		optionPagination,
		option.Query("name", "Filter by name", param.Example("cat name", "felix"), param.Nullable()),
		option.QueryInt("younger_than", "Only get pets younger than given age in years", param.Default(3)),
		option.Description("Filter pets"),
	)

	fuego.Get(petsGroup, "/all", rs.getAllPets,
		optionPagination,
		option.Tags("my-tag"),
		option.Description("Get all pets"),
	)

	fuego.Get(petsGroup, "/by-age", rs.getAllPetsByAge,
		option.Description("Returns an array of pets grouped by age"),
		option.Middleware(dummyMiddleware),
	)
	fuego.Post(petsGroup, "/", rs.postPets,
		option.DefaultStatusCode(201),
		option.AddResponse(409, "Conflict: Pet with the same name already exists", fuego.Response{Type: PetsError{}}),
	)

	fuego.Get(petsGroup, "/{id}", rs.getPets,
		option.OverrideDescription("Replace description with this sentence."),
		option.OperationID("getPet"),
		option.Path("id", "Pet ID", param.Example("example", "123")),
	)
	fuego.Get(petsGroup, "/by-name/{name...}", rs.getPetByName)
	fuego.Put(petsGroup, "/{id}", rs.putPets)
	fuego.Put(petsGroup, "/{id}/json", rs.putPets,
		option.Summary("Update a pet with JSON-only body"),
		option.RequestContentType("application/json"),
	)
	fuego.Delete(petsGroup, "/{id}", rs.deletePets)

	stdPetsGroup := fuego.Group(petsGroup, "/std")

	fuego.GetStd(stdPetsGroup, "/all", func(w http.ResponseWriter, r *http.Request) {
		pets, err := rs.PetsService.GetAllPets()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(pets); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}, option.AddResponse(http.StatusOK, "all the pets",
		fuego.Response{
			Type:         []models.Pets{},
			ContentTypes: []string{"application/json"},
		},
	))
}

func (rs PetsResources) getAllPets(c fuego.ContextNoBody) ([]models.Pets, error) {
	page := c.QueryParamInt("page")
	pageWithTypo := c.QueryParamInt("page-with-typo") // this shows a warning in the logs because "page-with-typo" is not a declared query param
	slog.Info("query params", "page", page, "page-with-typo", pageWithTypo)
	return rs.PetsService.GetAllPets()
}

func (rs PetsResources) filterPets(c fuego.ContextNoBody) ([]models.Pets, error) {
	return rs.PetsService.FilterPets(PetsFilter{
		Name:        c.QueryParam("name"),
		YoungerThan: c.QueryParamInt("younger_than"),
	})
}

func (rs PetsResources) getAllPetsByAge(c fuego.ContextNoBody) ([][]models.Pets, error) {
	return rs.PetsService.GetAllPetsByAge()
}

func (rs PetsResources) postPets(c fuego.ContextWithBody[models.PetsCreate]) (models.Pets, error) {
	body, err := c.Body()
	if err != nil {
		return models.Pets{}, err
	}

	return rs.PetsService.CreatePets(body)
}

func (rs PetsResources) getPets(c fuego.ContextNoBody) (models.Pets, error) {
	id := c.PathParam("id")

	return rs.PetsService.GetPets(id)
}

func (rs PetsResources) getPetByName(c fuego.ContextNoBody) (models.Pets, error) {
	name := c.PathParam("name")

	return rs.PetsService.GetPetByName(name)
}

func (rs PetsResources) putPets(c fuego.ContextWithBody[models.PetsUpdate]) (models.Pets, error) {
	id := c.PathParam("id")

	body, err := c.Body()
	if err != nil {
		return models.Pets{}, err
	}

	return rs.PetsService.UpdatePets(id, body)
}

func (rs PetsResources) deletePets(c fuego.ContextNoBody) (any, error) {
	return rs.PetsService.DeletePets(c.PathParam("id"))
}

type PetsFilter struct {
	Name        string
	YoungerThan int
}

type PetsService interface {
	GetPets(id string) (models.Pets, error)
	GetPetByName(name string) (models.Pets, error)
	CreatePets(models.PetsCreate) (models.Pets, error)
	GetAllPets() ([]models.Pets, error)
	FilterPets(PetsFilter) ([]models.Pets, error)
	GetAllPetsByAge() ([][]models.Pets, error)
	UpdatePets(id string, input models.PetsUpdate) (models.Pets, error)
	DeletePets(id string) (any, error)
}
