# CRUD

How to write simple CRUD operations.

This tutorial relies on the previous Hello world tutorial [Start from scratch.](/docs/tutorials/hello-world#start-from-scratch)

## Generation

```bash
go install github.com/go-fuego/fuego/cmd/fuego@latest
fuego controller books

# or in one line:
go run github.com/go-fuego/fuego/cmd/fuego@latest controller books
```

This generates a controller for the `books` resource.

:::tip

Use `fuego controller --with-service books` to generate a simple map based in memory service so you can pass it to the controller resources to quickly interact with your new book entity!

:::

You then have to implement the service interface in the controller to be able
to play with data. It's a form of **dependency injection** that we chose to use
for the code generator of Fuego, but you can implement it in any way you want.

To implement the service, you need to slightly modify the
generated `controller/books.go` and `main.go` files.

```go title="controller/books.go" {8-9,28-42} showLineNumbers
package controller

import (
	"github.com/go-fuego/fuego"
)

type BooksResources struct {
	// Use a concrete struct that implements the service (BooksService -> RealBooksService)
	BooksService RealBooksService
}

type Books struct {
	ID   string `json:"id"`


// ....
// ....

type BooksService interface {
	GetBooks(id string) (Books, error)
	CreateBooks(BooksCreate) (Books, error)
	GetAllBooks() ([]Books, error)
	UpdateBooks(id string, input BooksUpdate) (Books, error)
	DeleteBooks(id string) (any, error)
}


// Implement the BooksService interface
type RealBooksService struct {
	// Embed the interface to satisfy it.
	// This pattern is just there to make the code compile but you should implement all methods.
	BooksService
}

func (s RealBooksService) GetBooks(id string) (Books, error) {
	return Books{
		ID:   id,
		Name: "Test book data",
	}, nil
}

// TODO: Other BooksService interface implementations

// END OF CODE BLOCK
```

Then we'll inject this controller into the server.

```go title="main.go" {6-7,14-21}
package main

import (
	"github.com/go-fuego/fuego"

	// ADD NEXT LINE
	"hello-fuego/controller"
)

func main() {
	s := fuego.NewServer()
	// ....

	// Declare the resource
	booksResources := controller.BooksResources{
		BooksService: controller.RealBooksService{},
		// Other services & dependencies, like a DB etc.
	}

	// Plug the controllers into the server
	booksResources.Routes(s)

	s.Run()
}
```

If you've followed this far, `/books/:id` (GetBooks) has been implemented. 🥳

The generator will create the following routes:

- `GET /books`: list all books
- `POST /books`: create a new book
- `GET /books/:id`: get a book by id
- `PUT /books/:id`: update a book by id
- `PATCH /books/:id`: update a book by id
- `DELETE /books/:id`: delete a book by id

## Manual

:::tip

Fuego comes with a [**generator**](#generation)
that can generates CRUD routes and controllers for you!

:::

```go title="main.go"
package main

import (
	"github.com/go-fuego/fuego"

	"hello-fuego/controller"
)

func main() {
	s := fuego.NewServer()

	// List all books
	fuego.Get(s, "/books", controller.GetBooks)

	// Create a new book
	fuego.Post(s, "/books", controller.CreateBook)

	// Get a book by id
	fuego.Get(s, "/books/:id", controller.GetBook)

	// Update a book by id
	fuego.Put(s, "/books/:id", controller.UpdateBook)

	// Update a book by id
	fuego.Patch(s, "/books/:id", controller.UpdateBook)

	// Delete a book by id
	fuego.Delete(s, "/books/:id", controller.DeleteBook)

	s.Run()
}
```

```go title="controller/books.go"
package controller

import (
	"github.com/go-fuego/fuego"
)

type Book struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type BookToCreate struct {
	Title string `json:"title"`
}

func GetBooks(c fuego.ContextNoBody) ([]Book, error) {
	// Your code here
	return nil, nil
}

func CreateBook(c fuego.ContextWithBody[BookToCreate]) (Book, error) {
	// Your code here
	return Book{}, nil
}

func GetBook(c fuego.ContextNoBody) (Book, error) {
	// Your code here
	return Book{}, nil
}

func UpdateBook(c fuego.ContextWithBody[Book]) (Book, error) {
	// Your code here
	return Book{}, nil
}

func DeleteBook(c fuego.ContextNoBody) (any, error) {
	// Your code here
	return nil, nil
}

```
