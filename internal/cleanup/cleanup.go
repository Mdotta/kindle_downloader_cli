package cleanup

import (
	"fmt"
	"os"
)

func Dir(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}
	return nil
}
