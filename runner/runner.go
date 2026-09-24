package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/UNHCSC/organesson/db"
)

type (
	LogFunc func(stream db.JobLogStream, line string) error

	Config struct {
		WorkRoot       string
		StateRoot      string
		AllowedSources []string
		OpenTofuBinary string
		AnsibleBinary  string
		Timeout        time.Duration
		MaxOutputBytes int
	}

	Command struct {
		Executable  string
		Arguments   []string
		Directory   string
		Environment map[string]string
	}

	Result struct {
		ExitCode    int
		OutputBytes int
		Truncated   bool
		StartedAt   time.Time
		FinishedAt  time.Time
	}

	Executor interface {
		Run(ctx context.Context, command Command, log LogFunc) (Result, error)
	}

	LocalExecutor struct {
		config Config
	}
)

// NewLocalExecutor validates and creates a bounded local process executor.
func NewLocalExecutor(configuration Config) (executorResult *LocalExecutor, errResult error) {
	if configuration.Timeout <= 0 || configuration.MaxOutputBytes <= 0 {
		return nil, fmt.Errorf("runner timeout and output limit must be positive")
	}
	if configuration.WorkRoot, errResult = secureRoot(configuration.WorkRoot); errResult != nil {
		return nil, fmt.Errorf("work root: %w", errResult)
	}
	if configuration.StateRoot, errResult = secureRoot(configuration.StateRoot); errResult != nil {
		return nil, fmt.Errorf("state root: %w", errResult)
	}
	for index, source := range configuration.AllowedSources {
		if remoteSource(source) {
			configuration.AllowedSources[index] = strings.TrimSpace(source)
			continue
		}
		if configuration.AllowedSources[index], errResult = existingRoot(source); errResult != nil {
			return nil, fmt.Errorf("source root: %w", errResult)
		}
	}
	executorResult = &LocalExecutor{config: configuration}
	return
}

// Run executes explicit arguments in a clean environment and kills the process group on cancellation.
func (executor *LocalExecutor) Run(ctx context.Context, command Command, log LogFunc) (result Result, errResult error) {
	if executor == nil {
		return result, fmt.Errorf("runner is not configured")
	}
	if command.Executable == "" || strings.ContainsRune(command.Executable, os.PathSeparator) && !filepath.IsAbs(command.Executable) {
		return result, fmt.Errorf("executable must be a name or absolute path")
	}
	if command.Directory, errResult = executor.resolveWorkDirectory(command.Directory); errResult != nil {
		return
	}
	var timeoutContext context.Context
	var cancel context.CancelFunc
	timeoutContext, cancel = context.WithTimeout(ctx, executor.config.Timeout)
	defer cancel()
	// #nosec G204 -- the executable is configured or selected internally and arguments are passed without a shell.
	var process *exec.Cmd = exec.Command(command.Executable, command.Arguments...)
	process.Dir = command.Directory
	process.Env = cleanEnvironment(command.Environment)
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout io.ReadCloser
	var stderr io.ReadCloser
	if stdout, errResult = process.StdoutPipe(); errResult != nil {
		return
	}
	if stderr, errResult = process.StderrPipe(); errResult != nil {
		return
	}
	result.StartedAt = time.Now().UTC()
	if errResult = process.Start(); errResult != nil {
		return
	}
	var output *boundedOutput = &boundedOutput{limit: executor.config.MaxOutputBytes, log: log, secrets: commandSecrets(command.Environment)}
	var readers sync.WaitGroup
	readers.Add(2)
	go output.read(stdout, db.JobLogStreamStdout, &readers)
	go output.read(stderr, db.JobLogStreamStderr, &readers)
	var waited chan error = make(chan error, 1)
	go func() { waited <- process.Wait() }()
	select {
	case errResult = <-waited:
	case <-timeoutContext.Done():
		_ = syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
		<-waited
		errResult = timeoutContext.Err()
	}
	readers.Wait()
	result.FinishedAt = time.Now().UTC()
	result.OutputBytes, result.Truncated = output.count, output.truncated
	if process.ProcessState != nil {
		result.ExitCode = process.ProcessState.ExitCode()
	}
	if errResult != nil && !errors.Is(errResult, context.Canceled) && !errors.Is(errResult, context.DeadlineExceeded) {
		errResult = fmt.Errorf("command exited with code %d", result.ExitCode)
	}
	if errResult == nil {
		errResult = output.err()
	}
	return
}

func (executor *LocalExecutor) resolveWorkDirectory(path string) (pathResult string, errResult error) {
	if path == "" {
		return "", fmt.Errorf("working directory is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(executor.config.WorkRoot, path)
	}
	if pathResult, errResult = filepath.Abs(path); errResult != nil {
		return
	}
	var relative string
	if relative, errResult = filepath.Rel(executor.config.WorkRoot, pathResult); errResult != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("working directory escapes configured root")
	}
	if errResult = os.MkdirAll(pathResult, 0700); errResult != nil {
		return
	}
	var resolved string
	if resolved, errResult = filepath.EvalSymlinks(pathResult); errResult != nil {
		return
	}
	if relative, errResult = filepath.Rel(executor.config.WorkRoot, resolved); errResult != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("working directory symlink escapes configured root")
	}
	return resolved, nil
}

type boundedOutput struct {
	mu        sync.Mutex
	limit     int
	count     int
	truncated bool
	scanError error
	log       LogFunc
	secrets   []string
}

func (output *boundedOutput) read(reader io.Reader, stream db.JobLogStream, done *sync.WaitGroup) {
	defer done.Done()
	var scanner *bufio.Scanner = bufio.NewScanner(reader)
	var buffer []byte = make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		var line string = db.RedactSensitiveText(scanner.Text())
		for _, secret := range output.secrets {
			line = strings.ReplaceAll(line, secret, "[REDACTED]")
		}
		output.mu.Lock()
		var remaining int = output.limit - output.count
		if remaining <= 0 {
			output.truncated = true
			output.mu.Unlock()
			continue
		}
		if len(line) > remaining {
			line = line[:remaining]
			output.truncated = true
		}
		output.count += len(line)
		output.mu.Unlock()
		if output.log != nil && line != "" {
			_ = output.log(stream, line)
		}
	}
	if scanner.Err() != nil && !errors.Is(scanner.Err(), os.ErrClosed) {
		output.mu.Lock()
		if output.scanError == nil {
			output.scanError = fmt.Errorf("read command stream %v: %w", stream, scanner.Err())
		}
		output.mu.Unlock()
	}
}

func (output *boundedOutput) err() (errResult error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	errResult = output.scanError
	return
}

func cleanEnvironment(values map[string]string) []string {
	var environment []string = []string{"HOME=/nonexistent", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "PATH=/usr/local/bin:/usr/bin:/bin", "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1"}
	for key, value := range values {
		if key == "PATH" || key == "HOME" || strings.Contains(key, "\x00") || strings.Contains(value, "\x00") {
			continue
		}
		environment = append(environment, key+"="+value)
	}
	return environment
}

func commandSecrets(values map[string]string) (results []string) {
	for key, value := range values {
		var normalized string = strings.ToLower(key)
		if value != "" && (strings.Contains(normalized, "secret") || strings.Contains(normalized, "token") || strings.Contains(normalized, "password") || strings.Contains(normalized, "credential")) {
			results = append(results, value)
		}
	}
	return
}

func secureRoot(path string) (pathResult string, errResult error) {
	if pathResult, errResult = filepath.Abs(path); errResult != nil {
		return
	}
	if errResult = os.MkdirAll(pathResult, 0700); errResult != nil {
		return
	}
	// #nosec G302 -- this is a private directory and requires its owner execute bit for traversal.
	errResult = os.Chmod(pathResult, 0700)
	return
}

func existingRoot(path string) (pathResult string, errResult error) {
	if pathResult, errResult = filepath.Abs(path); errResult != nil {
		return
	}
	pathResult, errResult = filepath.EvalSymlinks(pathResult)
	return
}
