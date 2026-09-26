package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJSON(path string, v any) error {

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	defer os.Remove(tmp.Name())

	encoder := json.NewEncoder(tmp)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(v); err != nil {
		tmp.Close()
		return fmt.Errorf("encode: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	return fmt.Errorf("rename temp: %w", os.Rename(tmp.Name(), path))
}
