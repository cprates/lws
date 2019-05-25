package datastructs

import "errors"

var (
	// ErrEmptyList is for when trying to take data from an empty list.
	ErrEmptyList = errors.New("empty list")
	// ErrNoEnoughElems is for when trying to take more elements than the list contains.
	ErrNoEnoughElems = errors.New("no enough elements in the list")
)
