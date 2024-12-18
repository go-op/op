# Controllers

Controllers are the main way to interact with the application. They are responsible for handling the requests and responses.

## Controller types

### Returning JSON

```go
func (c fuego.ContextNoBody) (MyResponse, error)
```

Used when the request does not have a body. The response will be automatically serialized to JSON.

```go
func (c fuego.ContextWithBody[MyInput]) (MyResponse, error)
```

Used when the request has a body.
Fuego will automatically parse the body and validate it using the input struct.

### Returning HTML

```go
func (c fuego.ContextNoBody) (fuego.HTML, error)
```

Some special interface return types are used by Fuego to return special responses.

- `fuego.HTML` is used to return HTML responses from `html/template`.
- `fuego.Templ` is used to return HTML responses from `a-h/templ`.
- `fuego.Gomponent` is used to return HTML responses from `maragudk/gomponent`.

### Example of a JSON controller

```go
type MyInput struct {
	Name string `json:"name"`
}

type MyResponse struct {
	Message string `json:"message"`
}

func MyController(c fuego.ContextWithBody[MyInput]) (MyResponse, error) {
	body, err := c.Body()
	if err != nil {
		return MyResponse{}, err
	}

	return MyResponse{
		Message: "Hello " + body.Name,
	}, nil
}
```

## Headers

You can always go further in the request and response by using the underlying net/http request and response, by using `c.Request` and `c.Response`.

### Get request header

```go
func MyController(c fuego.ContextNoBody) (MyResponse, error) {
	value := c.Header("X-My-Header")
	return MyResponse{}, nil
}
```

### Set response header

```go
func MyController(c fuego.ContextNoBody) (MyResponse, error) {
	c.SetHeader("X-My-Header", "value")
	return MyResponse{}, nil
}
```

## Cookies

### Get request cookie

```go
func MyController(c fuego.ContextNoBody) (MyResponse, error) {
	value := c.Cookie("my-cookie")
	return MyResponse{}, nil
}
```

### Set response cookie

```go
func MyController(c fuego.ContextNoBody) (MyResponse, error) {
	c.SetCookie("my-cookie", "value")
	return MyResponse{}, nil
}
```
