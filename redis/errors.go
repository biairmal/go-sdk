package redis

import "errors"

var (
	ErrKeyNotFound = errors.New("redis: key not found")
	ErrNilValue    = errors.New("redis: nil value")
)
