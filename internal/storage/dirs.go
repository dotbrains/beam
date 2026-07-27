package storage

import (
	"fmt"
	"os"
)

func mkdirAll(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}
	return nil
}
