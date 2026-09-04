package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRealInstalledToolsRunHarmlessExample(t *testing.T) {
	if os.Getenv("ORGANESSON_RUNNER_SMOKE") != "1" {
		t.Skip("set ORGANESSON_RUNNER_SMOKE=1 to run installed OpenTofu and Ansible")
	}
	var (
		exampleRoot string
		err         error
	)
	if exampleRoot, err = filepath.Abs("../examples/runner"); err != nil {
		t.Fatal(err)
	}
	var temporary string = t.TempDir()
	var executor *LocalExecutor
	if executor, err = NewLocalExecutor(Config{WorkRoot: filepath.Join(temporary, "work"), StateRoot: filepath.Join(temporary, "state"), AllowedSources: []string{exampleRoot}, OpenTofuBinary: "tofu", AnsibleBinary: "ansible-playbook", Timeout: time.Minute, MaxOutputBytes: 1024 * 1024}); err != nil {
		t.Fatal(err)
	}
	var tools *ToolRunner
	if tools, err = NewToolRunner(executor); err != nil {
		t.Fatal(err)
	}
	var (
		tofuSource string = filepath.Join(exampleRoot, "opentofu")
		tofuDigest string
	)
	if tofuDigest, err = executor.DigestAllowedSource(tofuSource); err != nil {
		t.Fatal(err)
	}
	if _, err = executor.MaterializeSource(context.Background(), tofuSource+"?ref="+tofuDigest, "smoke/tofu", nil); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(temporary, "work", "smoke", "tofu", "variables.tfvars.json"), []byte(`{"deployment_id":1,"resources":[],"allocations":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = tools.Plan(context.Background(), "smoke/tofu", "variables.tfvars.json", nil); err != nil {
		t.Fatalf("OpenTofu plan: %v", err)
	}
	var planSummary PlanSummary
	if planSummary, err = tools.PlanSummary(context.Background(), "smoke/tofu", "planned.tfplan"); err != nil || planSummary.Add != 1 {
		t.Fatalf("OpenTofu summary: %#v err=%v", planSummary, err)
	}
	if _, err = tools.Apply(context.Background(), "smoke/tofu", "planned.tfplan", nil); err != nil {
		t.Fatalf("OpenTofu apply: %v", err)
	}
	var (
		ansibleSource string = filepath.Join(exampleRoot, "ansible")
		ansibleDigest string
	)
	if ansibleDigest, err = executor.DigestAllowedSource(ansibleSource); err != nil {
		t.Fatal(err)
	}
	if _, err = executor.MaterializeSource(context.Background(), ansibleSource+"?ref="+ansibleDigest, "smoke/ansible", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = tools.Run(context.Background(), "smoke/ansible", "inventory.yml", "site.yml", "", true, nil); err != nil {
		t.Fatalf("Ansible check: %v", err)
	}
}
