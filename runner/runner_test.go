package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/db"
)

func TestLocalExecutorRunsWithRedactionAndBounds(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, time.Second, 40)
	var executable string = writeExecutable(t, "output", "#!/bin/sh\nprintf '%s\\n' 'token=super-secret' \"$API_TOKEN\" 'ordinary-output-that-is-long-enough-to-truncate'\n")
	var lines []string
	var result Result
	var err error
	if result, err = executor.Run(context.Background(), Command{Executable: executable, Directory: "job-1", Environment: map[string]string{"API_TOKEN": "bare-secret-value"}}, func(_ db.JobLogStream, line string) error { lines = append(lines, line); return nil }); err != nil {
		t.Fatal(err)
	}
	var output string = strings.Join(lines, "\n")
	if strings.Contains(output, "super-secret") || strings.Contains(output, "bare-secret-value") || !strings.Contains(output, "[REDACTED]") || !result.Truncated || result.OutputBytes > 40 {
		t.Fatalf("unsafe bounded output result=%#v output=%q", result, output)
	}
}

func TestLocalExecutorCancellationKillsProcessGroup(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, 10*time.Second, 1024)
	var childFile string = filepath.Join(t.TempDir(), "child.pid")
	var executable string = writeExecutable(t, "cancel", "#!/bin/sh\nsleep 30 &\nprintf '%s' \"$!\" > \"$CHILD_PID_FILE\"\nwait\n")
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	var done chan error = make(chan error, 1)
	go func() {
		var runErr error
		_, runErr = executor.Run(ctx, Command{Executable: executable, Directory: "job-cancel", Environment: map[string]string{"CHILD_PID_FILE": childFile}}, nil)
		done <- runErr
	}()
	var body []byte
	var err error
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if body, err = os.ReadFile(childFile); err == nil && len(body) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	var pid int
	if pid, err = strconv.Atoi(string(body)); err != nil {
		t.Fatalf("read child PID: %v", err)
	}
	var state []byte
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if state, err = os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); os.IsNotExist(err) || strings.Contains(string(state), ") Z ") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child process %d survived cancellation: %s", pid, state)
}

func TestRunnerRejectsPathEscapeAndRunsToolArguments(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, time.Second, 4096)
	var executable string = writeExecutable(t, "tofu", "#!/bin/sh\nprintf '%s\\n' \"$*\"\nexit 0\n")
	executor.config.OpenTofuBinary = executable
	executor.config.AnsibleBinary = executable
	var tools *ToolRunner
	var err error
	if tools, err = NewToolRunner(executor); err != nil {
		t.Fatal(err)
	}
	var versions Versions
	if versions, err = tools.ToolVersions(context.Background()); err != nil || versions.OpenTofu != "version" || versions.Ansible != "--version" {
		t.Fatalf("unexpected fake tool versions: %#v err=%v", versions, err)
	}
	if _, err = tools.Plan(context.Background(), "../escape", "vars.tfvars.json", nil); err == nil {
		t.Fatal("expected workspace escape rejection")
	}
	if _, err = tools.Plan(context.Background(), "deployment-1", "../vars.tfvars.json", nil); err == nil {
		t.Fatal("expected variable path escape rejection")
	}
	if _, err = tools.Plan(context.Background(), "deployment-1", "vars.tfvars.json", nil); err != nil {
		t.Fatalf("run fake OpenTofu plan: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(executor.config.StateRoot, "deployment-1"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(executor.config.StateRoot, "deployment-1", "planned.tfplan"), []byte("plan"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = tools.Apply(context.Background(), "deployment-1", "planned.tfplan", nil); err != nil {
		t.Fatalf("run fake OpenTofu apply: %v", err)
	}
	if _, err = tools.Run(context.Background(), "deployment-1", "inventory.yml", "site.yml", "", true, nil); err != nil {
		t.Fatalf("run fake Ansible check: %v", err)
	}
}

func TestPlanSummaryRejectsMalformedOutput(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, time.Second, 4096)
	var executable string = writeExecutable(t, "tofu-malformed", "#!/bin/sh\nprintf '%s\\n' 'not-json'\n")
	executor.config.OpenTofuBinary = executable
	var tools *ToolRunner
	var err error
	if tools, err = NewToolRunner(executor); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(executor.config.StateRoot, "deployment-summary"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(executor.config.StateRoot, "deployment-summary", "planned.tfplan"), []byte("plan"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = tools.PlanSummary(context.Background(), "deployment-summary", "planned.tfplan"); err == nil || !strings.Contains(err.Error(), "invalid OpenTofu plan summary") {
		t.Fatalf("expected malformed summary error, got %v", err)
	}
}

func TestLocalExecutorReportsFailureAndTimeout(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, 50*time.Millisecond, 4096)
	var err error
	if _, err = executor.Run(context.Background(), Command{Executable: writeExecutable(t, "failure", "#!/bin/sh\nexit 7\n"), Directory: "failure"}, nil); err == nil || !strings.Contains(err.Error(), "code 7") {
		t.Fatalf("expected exit failure, got %v", err)
	}
	if _, err = executor.Run(context.Background(), Command{Executable: writeExecutable(t, "timeout", "#!/bin/sh\nsleep 30\n"), Directory: "timeout"}, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestMaterializeLocalSourceRequiresAllowlistAndDigest(t *testing.T) {
	var executor *LocalExecutor = newTestExecutor(t, time.Second, 4096)
	var source string = executor.config.AllowedSources[0]
	var err error
	if err = os.WriteFile(filepath.Join(source, "main.tf"), []byte("terraform {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var digest string
	if digest, err = directoryDigest(source); err != nil {
		t.Fatal(err)
	}
	if _, err = executor.MaterializeLocalSource(source+"?ref="+digest, "deployment/source"); err != nil {
		t.Fatalf("materialize pinned source: %v", err)
	}
	if _, err = os.Stat(filepath.Join(executor.config.WorkRoot, "deployment", "source", "main.tf")); err != nil {
		t.Fatal("source was not copied into workspace")
	}
	if _, err = executor.MaterializeLocalSource(source+"?ref=sha256:bad", "deployment/bad"); err == nil {
		t.Fatal("expected digest mismatch rejection")
	}
	if _, err = executor.MaterializeLocalSource(t.TempDir()+"?ref="+digest, "deployment/outside"); err == nil {
		t.Fatal("expected source allowlist rejection")
	}
}

func newTestExecutor(t *testing.T, timeout time.Duration, outputLimit int) (executorResult *LocalExecutor) {
	t.Helper()
	var root string = t.TempDir()
	var source string = filepath.Join(root, "sources")
	var err error
	if err = os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if executorResult, err = NewLocalExecutor(Config{WorkRoot: filepath.Join(root, "work"), StateRoot: filepath.Join(root, "state"), AllowedSources: []string{source}, OpenTofuBinary: "tofu", AnsibleBinary: "ansible-playbook", Timeout: timeout, MaxOutputBytes: outputLimit}); err != nil {
		t.Fatal(err)
	}
	return
}

func writeExecutable(t *testing.T, name, body string) (pathResult string) {
	t.Helper()
	pathResult = filepath.Join(t.TempDir(), name)
	var err error
	if err = os.WriteFile(pathResult, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	return
}
