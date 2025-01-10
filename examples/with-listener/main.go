package main

import (
	"net"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	s := fuego.NewServer(fuego.WithListener(listener))

	fuego.Get(s, "/", helloWorld,
		option.Summary("A simple hello world"),
		option.Description("This is a simple hello world"),
		option.Deprecated(),
	)

	s.Run()
}

func helloWorld(c fuego.ContextNoBody) (string, error) {
	return "Hello, World!", nil
}
