package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
)

func loadConfig(path string) (err error) {
	// Apply struct defaults BEFORE loading TOML (so TOML overrides)
	if err = defaults.Set(&Config); err != nil {
		err = fmt.Errorf("set defaults: %w", err)
		return
	}

	// Decode TOML file into struct
	if _, err = toml.DecodeFile(path, &Config); err != nil {
		err = fmt.Errorf("decode toml: %w", err)
		return
	}

	// Validate required fields
	if err = validator.New(validator.WithRequiredStructEnabled()).Struct(Config); err != nil {
		err = fmt.Errorf("validate config: %w", err)
		return
	}

	return
}

// generateDefaultConfig writes a config.toml with all default values filled in.
// It will overwrite any existing file at path.
func generateDefaultConfig(path string) (err error) {
	var (
		cfg          Configuration
		absolutePath string
		root         *os.Root
		file         *os.File
	)

	// 1. Apply struct defaults
	if err = defaults.Set(&cfg); err != nil {
		err = fmt.Errorf("set defaults: %w", err)
		return
	}

	// NOTE: Do NOT validate here.
	// The default config is allowed to be "invalid" from a required-fields POV;
	// it's just a template for the user to fill in.
	// Validation happens in LoadConfig() when we actually load the file.

	// 2. Create / truncate the file within its explicitly configured parent.
	if absolutePath, err = filepath.Abs(path); err != nil {
		err = fmt.Errorf("resolve config path: %w", err)
		return
	}

	if root, err = os.OpenRoot(filepath.Dir(absolutePath)); err != nil {
		err = fmt.Errorf("open config directory: %w", err)
		return
	}
	defer func() {
		_ = root.Close()
	}()

	if file, err = root.OpenFile(filepath.Base(absolutePath), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600); err != nil {
		err = fmt.Errorf("create config file: %w", err)
		return
	}
	defer func() {
		var closeErr error

		if closeErr = file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close config file: %w", closeErr)
		}
	}()

	if err = file.Chmod(0600); err != nil {
		err = fmt.Errorf("secure config file permissions: %w", err)
		return
	}

	// 3. Encode as TOML
	var encoder *toml.Encoder = toml.NewEncoder(file)
	encoder.Indent = "    "
	if err = encoder.Encode(cfg); err != nil {
		err = fmt.Errorf("encode toml: %w", err)
	}

	return
}

// Init initializes this package.
func Init(path string) (err error) {
	if !filepath.IsAbs(path) {
		if path, err = filepath.Abs(path); err != nil {
			return err
		}
	}

	if _, err = os.Stat(path); err != nil {
		if err = generateDefaultConfig(path); err != nil {
			return
		}

		err = fmt.Errorf("no config file found, created a default config at %s. Please fill in the required values and try again", path)
		return
	}

	if err = loadConfig(path); err != nil {
		return err
	}

	return nil
}
