package repository

import (
	"errors"
)

var (
	// ErrNotFound is returned when entity is not found.
	ErrNotFound = errors.New("repository: entity not found")

	// ErrAlreadyExists is returned when entity already exists.
	ErrAlreadyExists = errors.New("repository: entity already exists")

	// ErrInvalidID is returned when ID format is invalid.
	ErrInvalidID = errors.New("repository: invalid ID")

	// ErrInvalidEntity is returned when entity validation fails.
	ErrInvalidEntity = errors.New("repository: invalid entity")

	// ErrConflict is returned when update conflicts with existing data.
	ErrConflict = errors.New("repository: update conflict")

	// ErrConnection is returned when database connection fails.
	ErrConnection = errors.New("repository: connection error")
)

// IsNotFound checks if error is ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if error is ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsConflict checks if error is ErrConflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
