package main

import "github.com/alok-mishra143/hades/internal/store"

func main() {
	s := store.NewStore()
	s.Set("alok", "newalok")
	val, _ := s.Get("alok")

	println(val)

}
