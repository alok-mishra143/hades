package store

import "errors"

var (
	ErrNoSuchKey   = errors.New("ERR no such key")
	ErrKeyExists   = errors.New("ERR target key already exists")
	ErrOOM         = errors.New("OOM store is full")
	ErrTimeCorrect = errors.New("You need to Enter correct time")
)
