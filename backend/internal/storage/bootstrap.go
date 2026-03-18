package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"openshare/backend/internal/config"
)

func EnsureLayout(cfg config.StorageConfig) error {
	for _, path := range []string{
		cfg.Root,
		stagingPath(cfg),
		trashPath(cfg),
	} {
		if err := ensureDir(path); err != nil {
			return err
		}
	}

	return nil
}

func stagingPath(cfg config.StorageConfig) string {
	return filepath.Join(cfg.Root, cfg.Staging)
}

func trashPath(cfg config.StorageConfig) string {
	return filepath.Join(cfg.Root, cfg.Trash)
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create storage directory %q: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat storage directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("storage path %q is not a directory", path)
	}

	if err := probeWritable(path); err != nil {
		return err
	}

	return nil
}

func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".openshare-probe-*")
	if err != nil {
		return fmt.Errorf("write permission check failed for %q: %w", dir, err)
	}
	name := f.Name()
	_ = f.Close()

	if err := os.Remove(name); err != nil {
		return fmt.Errorf("cleanup permission check failed for %q: %w", dir, err)
	}

	return nil
}
