package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UNHCSC/organesson/db"
)

type (
	OpenTofu interface {
		Health(ctx context.Context, log LogFunc) error
		Plan(ctx context.Context, workspace string, variablesFile string, log LogFunc) (Result, error)
		Apply(ctx context.Context, workspace string, planFile string, log LogFunc) (Result, error)
		Destroy(ctx context.Context, workspace string, variablesFile string, log LogFunc) (Result, error)
		PlanSummary(ctx context.Context, workspace string, planFile string) (PlanSummary, error)
	}

	Ansible interface {
		Health(ctx context.Context, log LogFunc) error
		Run(ctx context.Context, workspace string, inventoryFile string, playbookFile string, extraVariablesFile string, check bool, log LogFunc) (Result, error)
	}

	ToolRunner struct {
		executor *LocalExecutor
	}

	PlanSummary struct {
		Add     int `json:"add"`
		Change  int `json:"change"`
		Destroy int `json:"destroy"`
	}

	Versions struct {
		OpenTofu string `json:"opentofu"`
		Ansible  string `json:"ansible"`
	}
)

// NewToolRunner creates OpenTofu and Ansible adapters over the bounded executor.
func NewToolRunner(executor *LocalExecutor) (runnerResult *ToolRunner, errResult error) {
	if executor == nil {
		return nil, fmt.Errorf("executor is required")
	}
	return &ToolRunner{executor: executor}, nil
}

// Health checks that OpenTofu is executable without inheriting the server environment.
func (runner *ToolRunner) Health(ctx context.Context, log LogFunc) (errResult error) {
	return runner.OpenTofuHealth(ctx, log)
}

// OpenTofuHealth reports whether the configured OpenTofu executable runs.
func (runner *ToolRunner) OpenTofuHealth(ctx context.Context, log LogFunc) (errResult error) {
	_, errResult = runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"version"}, Directory: "health/tofu"}, log)
	return
}

// AnsibleHealth reports whether the configured Ansible executable runs.
func (runner *ToolRunner) AnsibleHealth(ctx context.Context, log LogFunc) (errResult error) {
	var environment map[string]string
	if environment, errResult = runner.ansibleEnvironment("health/ansible"); errResult != nil {
		return
	}
	_, errResult = runner.executor.Run(ctx, Command{Executable: runner.executor.config.AnsibleBinary, Arguments: []string{"--version"}, Directory: "health/ansible", Environment: environment}, log)
	return
}

// ToolVersions returns the first version line reported by each configured tool.
func (runner *ToolRunner) ToolVersions(ctx context.Context) (versions Versions, errResult error) {
	if versions.OpenTofu, errResult = runner.versionLine(ctx, runner.executor.config.OpenTofuBinary, []string{"version"}, "health/tofu-version", nil); errResult != nil {
		return
	}
	var environment map[string]string
	if environment, errResult = runner.ansibleEnvironment("health/ansible-version"); errResult != nil {
		return
	}
	versions.Ansible, errResult = runner.versionLine(ctx, runner.executor.config.AnsibleBinary, []string{"--version"}, "health/ansible-version", environment)
	return
}

func (runner *ToolRunner) versionLine(ctx context.Context, executable string, arguments []string, workspace string, environment map[string]string) (versionResult string, errResult error) {
	var capture LogFunc = func(stream db.JobLogStream, line string) (err error) {
		if stream == db.JobLogStreamStdout && versionResult == "" {
			versionResult = strings.TrimSpace(line)
		}
		return
	}
	if _, errResult = runner.executor.Run(ctx, Command{Executable: executable, Arguments: arguments, Directory: workspace, Environment: environment}, capture); errResult != nil {
		return
	}
	if versionResult == "" {
		errResult = fmt.Errorf("tool returned no version")
	}
	return
}

// Plan initializes and writes a non-destructive OpenTofu plan in its deployment workspace.
func (runner *ToolRunner) Plan(ctx context.Context, workspace string, variablesFile string, log LogFunc) (result Result, errResult error) {
	if workspace, errResult = cleanRelativeWorkspace(workspace); errResult != nil {
		return
	}
	if variablesFile, errResult = safeWorkspaceFile(variablesFile, ".tfvars.json"); errResult != nil {
		return
	}
	if _, errResult = runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"init", "-input=false", "-no-color"}, Directory: workspace}, log); errResult != nil {
		return
	}
	var stateDirectory string = filepath.Join(runner.executor.config.StateRoot, workspace)
	if errResult = os.MkdirAll(stateDirectory, 0700); errResult != nil {
		return
	}
	result, errResult = runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"plan", "-input=false", "-no-color", "-lock=true", "-out=" + filepath.Join(stateDirectory, "planned.tfplan"), "-var-file=" + variablesFile}, Directory: workspace}, log)
	return
}

// Apply executes only a previously generated plan file.
func (runner *ToolRunner) Apply(ctx context.Context, workspace string, planFile string, log LogFunc) (result Result, errResult error) {
	if workspace, errResult = cleanRelativeWorkspace(workspace); errResult != nil {
		return
	}
	if planFile, errResult = runner.safeStateFile(workspace, planFile, ".tfplan"); errResult != nil {
		return
	}
	return runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"apply", "-input=false", "-no-color", "-auto-approve", planFile}, Directory: workspace}, log)
}

// PlanSummary reads a saved plan and returns its resource action counts.
func (runner *ToolRunner) PlanSummary(ctx context.Context, workspace string, planFile string) (summary PlanSummary, errResult error) {
	if workspace, errResult = cleanRelativeWorkspace(workspace); errResult != nil {
		return
	}
	if planFile, errResult = runner.safeStateFile(workspace, planFile, ".tfplan"); errResult != nil {
		return
	}
	var output strings.Builder
	var capture LogFunc = func(stream db.JobLogStream, line string) (err error) {
		if stream == db.JobLogStreamStdout {
			output.WriteString(line)
		}
		return
	}
	if _, errResult = runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"show", "-json", planFile}, Directory: workspace}, capture); errResult != nil {
		return
	}
	var plan struct {
		ResourceChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if errResult = json.Unmarshal([]byte(output.String()), &plan); errResult != nil {
		errResult = fmt.Errorf("invalid OpenTofu plan summary: %w", errResult)
		return
	}
	for _, resource := range plan.ResourceChanges {
		for _, action := range resource.Change.Actions {
			switch action {
			case "create":
				summary.Add++
			case "update":
				summary.Change++
			case "delete":
				summary.Destroy++
			}
		}
	}
	return
}

// Destroy performs an explicitly requested OpenTofu destroy operation.
func (runner *ToolRunner) Destroy(ctx context.Context, workspace string, variablesFile string, log LogFunc) (result Result, errResult error) {
	if workspace, errResult = cleanRelativeWorkspace(workspace); errResult != nil {
		return
	}
	if variablesFile, errResult = safeWorkspaceFile(variablesFile, ".tfvars.json"); errResult != nil {
		return
	}
	return runner.executor.Run(ctx, Command{Executable: runner.executor.config.OpenTofuBinary, Arguments: []string{"destroy", "-input=false", "-no-color", "-auto-approve", "-var-file=" + variablesFile}, Directory: workspace}, log)
}

// Run executes an allowlisted inventory and playbook from one deployment workspace.
func (runner *ToolRunner) Run(ctx context.Context, workspace string, inventoryFile string, playbookFile string, extraVariablesFile string, check bool, log LogFunc) (result Result, errResult error) {
	if workspace, errResult = cleanRelativeWorkspace(workspace); errResult != nil {
		return
	}
	if inventoryFile, errResult = safeWorkspaceFile(inventoryFile, ""); errResult != nil {
		return
	}
	if playbookFile, errResult = safeWorkspaceFile(playbookFile, ".yml", ".yaml"); errResult != nil {
		return
	}
	var arguments []string = []string{"--inventory", inventoryFile, playbookFile}
	if extraVariablesFile != "" {
		if extraVariablesFile, errResult = safeWorkspaceFile(extraVariablesFile, ".json", ".yml", ".yaml"); errResult != nil {
			return
		}
		arguments = append(arguments, "--extra-vars", "@"+extraVariablesFile)
	}
	if check {
		arguments = append(arguments, "--check", "--diff")
	}
	var environment map[string]string
	if environment, errResult = runner.ansibleEnvironment(workspace); errResult != nil {
		return
	}
	return runner.executor.Run(ctx, Command{Executable: runner.executor.config.AnsibleBinary, Arguments: arguments, Directory: workspace, Environment: environment}, log)
}

func (runner *ToolRunner) ansibleEnvironment(workspace string) (environmentResult map[string]string, errResult error) {
	var temporaryDirectory string = filepath.Join(runner.executor.config.WorkRoot, workspace, ".ansible", "tmp")
	if errResult = os.MkdirAll(temporaryDirectory, 0700); errResult != nil {
		return
	}
	environmentResult = map[string]string{"ANSIBLE_NOCOLOR": "1", "ANSIBLE_HOST_KEY_CHECKING": "True", "ANSIBLE_LOCAL_TEMP": temporaryDirectory}
	return
}

func (runner *ToolRunner) safeStateFile(workspace, path string, extensions ...string) (pathResult string, errResult error) {
	if filepath.IsAbs(path) || filepath.Base(path) != path || !hasExtension(path, extensions) {
		return "", fmt.Errorf("state file must be a safe filename")
	}
	var root string = filepath.Join(runner.executor.config.StateRoot, workspace)
	pathResult = filepath.Join(root, path)
	return
}

func cleanRelativeWorkspace(value string) (valueResult string, errResult error) {
	value = filepath.Clean(strings.TrimSpace(value))
	if value == "." || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace must remain below the runner root")
	}
	return value, nil
}

func safeWorkspaceFile(value string, extensions ...string) (valueResult string, errResult error) {
	value = filepath.Clean(strings.TrimSpace(value))
	if value == "." || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, ".."+string(os.PathSeparator)) || !hasExtension(value, extensions) {
		return "", fmt.Errorf("file must remain below the deployment workspace")
	}
	return value, nil
}

func hasExtension(value string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	for _, extension := range extensions {
		if strings.HasSuffix(strings.ToLower(value), extension) {
			return true
		}
	}
	return false
}
