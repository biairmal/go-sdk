package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadEnvFile loads environment variables from the file at path and sets them
// in the process environment. Path is relative to the current working
// directory or absolute. If path is empty, LoadEnvFile does nothing and
// returns nil. If the file does not exist, an error is returned.
func LoadEnvFile(path string) error {
	if path == "" {
		return nil
	}
	return godotenv.Load(path)
}

// LoadEnvFileOptional loads environment variables from the file at path and
// sets them in the process environment. If the file does not exist, no error
// is returned (useful for optional .env in development). Path semantics are
// the same as LoadEnvFile.
func LoadEnvFileOptional(path string) error {
	if path == "" {
		return nil
	}
	err := godotenv.Load(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
