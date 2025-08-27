package main

import (
	"log"

	"github.com/Dyastin-0/gobyte"
)

func main() {
	s := gobyte.NewFileSelector("./")

	err := s.RunRecur()
	if err != nil {
		log.Printf("[err] %v\n", err)
	}

	for _, f := range s.Selected {
		log.Printf("%v\n", f)
	}
}
