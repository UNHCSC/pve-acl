// Package safefile provides traversal-safe access to explicitly configured files.
package safefile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Read reads a file without allowing its base name to escape its configured parent directory.
func Read(path string) (contentsResult []byte, errResult error) {
	var (
		name string
		root *os.Root
	)

	if root, name, errResult = openParent(path); errResult != nil {
		return
	}
	defer func() {
		_ = root.Close()
	}()

	if contentsResult, errResult = root.ReadFile(name); errResult != nil {
		errResult = fmt.Errorf("read file: %w", errResult)
	}

	return
}

// WritePrivate writes a file with owner-only permissions inside its configured parent directory.
func WritePrivate(path string, contents []byte) (errResult error) {
	var (
		file *os.File
		name string
		root *os.Root
	)

	if root, name, errResult = openParent(path); errResult != nil {
		return
	}
	defer func() {
		_ = root.Close()
	}()

	if file, errResult = root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600); errResult != nil {
		errResult = fmt.Errorf("open private file: %w", errResult)
		return
	}
	defer func() {
		var closeErr error

		if closeErr = file.Close(); errResult == nil && closeErr != nil {
			errResult = fmt.Errorf("close private file: %w", closeErr)
		}
	}()

	if errResult = file.Chmod(0600); errResult != nil {
		errResult = fmt.Errorf("secure private file permissions: %w", errResult)
		return
	}

	if _, errResult = file.Write(contents); errResult != nil {
		errResult = fmt.Errorf("write private file: %w", errResult)
	}

	return
}

func openParent(path string) (rootResult *os.Root, nameResult string, errResult error) {
	var absolutePath string

	if absolutePath, errResult = filepath.Abs(path); errResult != nil {
		errResult = fmt.Errorf("resolve file path: %w", errResult)
		return
	}

	if rootResult, errResult = os.OpenRoot(filepath.Dir(absolutePath)); errResult != nil {
		errResult = fmt.Errorf("open file directory: %w", errResult)
		return
	}

	nameResult = filepath.Base(absolutePath)
	return
}
