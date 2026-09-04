package runner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaterializeSource acquires an immutable local digest or remote Git commit from an allowlisted source.
func (executor *LocalExecutor) MaterializeSource(ctx context.Context, reference, workspace string, log LogFunc) (digestResult string, errResult error) {
	if !remoteSource(reference) {
		return executor.MaterializeLocalSource(reference, workspace)
	}
	var parts []string = strings.SplitN(strings.TrimSpace(reference), "?ref=", 2)
	if len(parts) != 2 || !immutableCommit(parts[1]) || !executor.remoteAllowed(parts[0]) {
		return "", fmt.Errorf("remote source must be allowlisted and pinned to a full Git commit")
	}
	if _, errResult = executor.resolveWorkDirectory(workspace); errResult != nil {
		return
	}
	var commands [][]string = [][]string{{"init"}, {"remote", "add", "origin", parts[0]}, {"fetch", "--depth=1", "origin", parts[1]}, {"checkout", "--detach", "FETCH_HEAD"}}
	for _, arguments := range commands {
		if _, errResult = executor.Run(ctx, Command{Executable: "git", Arguments: arguments, Directory: workspace, Environment: map[string]string{"GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0"}}, log); errResult != nil {
			return "", errResult
		}
	}
	return "git:" + strings.ToLower(parts[1]), nil
}

// DigestAllowedSource returns the content digest for an allowlisted local source.
func (executor *LocalExecutor) DigestAllowedSource(path string) (digestResult string, errResult error) {
	if path, errResult = filepath.Abs(path); errResult != nil {
		return
	}
	if path, errResult = filepath.EvalSymlinks(path); errResult != nil {
		return
	}
	if !executor.sourceAllowed(path) {
		return "", fmt.Errorf("source is outside configured roots")
	}
	return directoryDigest(path)
}

// MaterializeLocalSource copies a digest-pinned allowlisted source into a private workspace.
func (executor *LocalExecutor) MaterializeLocalSource(reference, workspace string) (digestResult string, errResult error) {
	var parts []string = strings.SplitN(strings.TrimSpace(reference), "?ref=sha256:", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("local source must use ?ref=sha256:<digest>")
	}
	var source string
	if source, errResult = filepath.Abs(parts[0]); errResult != nil {
		return
	}
	if source, errResult = filepath.EvalSymlinks(source); errResult != nil {
		return
	}
	if !executor.sourceAllowed(source) {
		return "", fmt.Errorf("source is outside configured roots")
	}
	if digestResult, errResult = directoryDigest(source); errResult != nil {
		return
	}
	if digestResult != "sha256:"+strings.ToLower(parts[1]) {
		return "", fmt.Errorf("source digest does not match pinned revision; current digest is %s", digestResult)
	}
	var destination string
	if destination, errResult = executor.resolveWorkDirectory(workspace); errResult != nil {
		return
	}
	if errResult = copySourceTree(source, destination); errResult != nil {
		return "", errResult
	}
	return
}

func (executor *LocalExecutor) sourceAllowed(source string) bool {
	for _, root := range executor.config.AllowedSources {
		if remoteSource(root) {
			continue
		}
		var relative string
		var err error
		if relative, err = filepath.Rel(root, source); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func (executor *LocalExecutor) remoteAllowed(source string) bool {
	for _, root := range executor.config.AllowedSources {
		var boundary string = strings.TrimRight(root, "/")
		if remoteSource(root) && (source == boundary || strings.HasPrefix(source, boundary+"/")) {
			return true
		}
	}
	return false
}

func remoteSource(value string) bool {
	return strings.Contains(value, "://") || strings.HasPrefix(value, "git@")
}

func immutableCommit(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func directoryDigest(root string) (valueResult string, errResult error) {
	var paths []string
	if errResult = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source contains a symlink")
		}
		if !entry.IsDir() {
			var relative string
			if relative, walkErr = filepath.Rel(root, path); walkErr != nil {
				return walkErr
			}
			paths = append(paths, filepath.ToSlash(relative))
		}
		return nil
	}); errResult != nil {
		return
	}
	sort.Strings(paths)
	var hash = sha256.New()
	for _, relative := range paths {
		var file *os.File
		if _, errResult = io.WriteString(hash, relative+"\x00"); errResult != nil {
			return
		}
		if file, errResult = os.Open(filepath.Join(root, filepath.FromSlash(relative))); errResult != nil {
			return
		}
		if _, errResult = io.Copy(hash, file); errResult != nil {
			_ = file.Close()
			return
		}
		if errResult = file.Close(); errResult != nil {
			return
		}
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

func copySourceTree(source, destination string) (errResult error) {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) (callbackErr error) {
		if walkErr != nil {
			return walkErr
		}
		var relative string
		if relative, callbackErr = filepath.Rel(source, path); callbackErr != nil {
			return
		}
		var target string = filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("source contains a non-regular file")
		}
		var input *os.File
		var output *os.File
		if input, callbackErr = os.Open(path); callbackErr != nil {
			return
		}
		defer input.Close()
		if output, callbackErr = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); callbackErr != nil {
			return
		}
		if _, callbackErr = io.Copy(output, input); callbackErr != nil {
			_ = output.Close()
			return
		}
		return output.Close()
	})
}
