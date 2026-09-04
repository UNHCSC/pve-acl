# Runner smoke example

This example is deliberately harmless: OpenTofu creates only a `terraform_data`
resource and Ansible connects only to localhost. It exists to exercise the real
runner before using Proxmox modules or lab inventories.

Blueprint source references must use the directory digest reported by the
runner and the form:

```text
/absolute/path/to/examples/runner/opentofu?ref=sha256:<digest>
/absolute/path/to/examples/runner/ansible?ref=sha256:<digest>
```

The source directory must also be below one of `runner.allowed_source_roots`.

Run the installed-tool smoke test from the repository root with:

```text
ORGANESSON_RUNNER_SMOKE=1 go test ./runner -run TestRealInstalledToolsRunHarmlessExample -count=1 -v
```

This test executes OpenTofu init, plan, structured plan inspection, and apply,
then executes the Ansible playbook in check mode. Its work and state directories
are temporary and are removed when the test finishes. It never contacts
Proxmox or another remote host.
