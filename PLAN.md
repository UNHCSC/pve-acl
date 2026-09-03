# Classroom Lab Delivery Plan

## 1. Outcome

Turn Organesson Cloud from its current identity, RBAC, local resource-registry, and scheduler foundation into a reasonably reliable classroom control plane for the lab in `goal.md`.

At the classroom-ready cutoff, an instructor or TA must be able to:

1. Create a course project, import students, and arrange them into groups of one or more.
2. Deploy one isolated copy of the lab for every group from a versioned blueprint.
3. See deployment progress and useful logs.
4. Assign each lab to its group without transferring project ownership.
5. Let students see, start, stop, reboot, and open consoles only for their lab VMs.
6. Run configuration and grading workflows against one or many labs.
7. Reset, redeploy, archive, and destroy labs safely.
8. Diagnose failures using job history, audit history, and reconciliation state.

Each deployment must contain the eight devices from `goal.md`: pfSense, two ArubaOS-CX switches, Active Directory, Windows 11, Fedora, a DMZ web server, and an Ubuntu DMZ client. It must support the discovery, firewall, VLAN, management-network, domain-join, Fedora-upgrade, public-web, segmentation, IPv6, and grading exercises described there.

“Classroom ready” means suitable for a supervised course on a known Proxmox cluster. It does not mean public-cloud-grade availability or arbitrary untrusted users.

## 2. Current Starting Point

Already present:

- LDAP/JWT authentication and local user synchronization.
- Organizations, projects, groups, memberships, roles, permissions, and scoped RBAC.
- Organization/project inheritance and resource-scoped grants.
- Local resource, asset-group, and asset-assignment database models.
- In-progress project resource, asset-group, and assignment APIs and UI.
- Proxmox inventory, quota, job, audit, and secret schema foundations.
- An embedded `gasket` scheduler with job status/log helpers and a tested no-op consumer.
- Passing Go tests, TypeScript checks, and client production build.

Not yet present:

- A working Proxmox service or live Proxmox operations.
- Inventory reconciliation.
- Job APIs, job UI, cancellation, or real task consumers.
- Terraform/OpenTofu or Ansible runners, modules, playbooks, or inventories.
- Lab blueprint/deployment models or collision-free network allocation.
- Browser console brokering.
- Consistent quotas, auditing, encrypted secret handling, and operational runbooks.

The current uncommitted work is the input to Slice 0. Do not discard or rewrite it wholesale.

## 3. Recommended Decisions

### Ownership

- One course/semester is an organization; one section or lab offering is a project.
- One student team is a local cloud group.
- One deployed lab is an asset group owned by the project and assigned to the student group.
- VMs and networks remain project-owned.
- Instructors/TAs receive project-scoped roles; students receive resource access through their lab assignment.

### Provisioning

- Prefer OpenTofu-compatible Terraform and call it “Terraform/OpenTofu” in the UI.
- Use it to clone and wire all eight VMs and create/select virtual networks.
- Use Ansible after provisioning for baseline setup, exercise preparation/reset, facts, and grading.
- Never execute infrastructure tooling in an HTTP handler. Every run is a queued job.
- Pin provider, module, template, playbook, and blueprint versions for a course run.

### Networking

- Reserve a configured WAN IPv4/IPv6 pool on the cybersecurity network.
- Give each lab unique WAN allocations and isolated LAN/DMZ virtual networks.
- Reuse private LAN/DMZ prefixes only if Proxmox networking guarantees isolation.
- Prefer a preconfigured SDN zone, bridge, or VLAN-aware trunk as the trusted substrate. Create per-lab VNets/VLANs/VXLANs only inside that boundary.
- Store allocations in the application database and enforce uniqueness transactionally.
- Make public web access an explicit blueprint option with deterministic NAT or routed allocation.

### Templates and secrets

- Validate golden templates for all eight devices before a course release.
- Identify them by cluster/template ID plus an immutable application version.
- Use cloud-init where supported and documented first-boot mechanisms elsewhere.
- Encrypt stored secrets with a deployment-provided master key.
- Never put secrets in job metadata, API responses, exposed Terraform state, or logs.

### Safety

- Permission-check mutations in the API and validate immutable identity/ownership again in workers.
- Require explicit confirmation for destructive bulk actions.
- Mutate/destroy only resources tagged with the expected Organesson deployment and project UUIDs.
- Block destroy when identity, tags, or state ownership is ambiguous.
- Use one Terraform state lock domain per deployment.
- Configure global and per-node concurrency to prevent class-wide clone storms.

## 4. Branch Rules

Each numbered slice is one reviewable branch, based on the preceding slice:

```text
classroom/00-foundation-hardening
classroom/01-quota-audit-secrets
classroom/02-proxmox-inventory
classroom/03-jobs-operations
classroom/04-vm-console
classroom/05-lab-blueprints
classroom/06-runner-foundation
classroom/07-lab-provisioning
classroom/08-lab-configuration
classroom/09-student-experience
classroom/10-grading-reset
classroom/11-pilot-hardening
classroom/12-honors-attacks
```

Every branch must:

- Include the schema/helpers, API, UI, and tests needed for its vertical user story.
- Keep Proxmox, runners, filesystem, and command execution behind interfaces with fakes.
- Use forward-compatible migrations and leave the app deployable.
- Pass:

```text
go test ./...
cd client && bun run check && bun run build
git diff --check
```

Real-cluster tests must be opt-in; fake-service integration tests remain part of the normal suite.

## 5. Implementation Slices

### Slice 0 — Stabilize Resources and Scheduler

Branch: `classroom/00-foundation-hardening`

Purpose: turn the current uncommitted work into a safe baseline.

Scope:

- Finish and commit the resource, asset-group, assignment, job-model, and scheduler work already present.
- Fix resource archive authorization: project visibility is insufficient; require project management or the correct delete permission.
- Add negative authorization tests for create, update, archive, assignment changes, and cross-project IDs.
- Ensure asset-group grants cover current and subsequently added resources and are safely removed on archive.
- Add asset-group update/archive API and UI.
- Make operational resource states server-owned; clients cannot arbitrarily set `ready`, `deleting`, or `error`.
- Limit this registry to VM, CT, and network until other resource types have real workflows.
- Ignore/remove runtime task databases such as `pve-acl.db.tasks` from version control.

Acceptance:

1. A manager creates local resources, groups them, and assigns the group to students.
2. Students see assigned resources but cannot update/archive them or change access.
3. The no-op scheduler test creates a job, executes it, and records logs.
4. All baseline checks pass and the work is committed coherently.

### Slice 1 — Quotas, Audit, Lifecycle, and Secrets

Branch: `classroom/01-quota-audit-secrets`

Purpose: add guardrails before live infrastructure mutations.

Scope:

- Implement quota-policy CRUD/bindings and effective quota resolution for organizations, projects, groups, and optionally users.
- Calculate intended VM, vCPU, RAM, disk, network, and public-address usage.
- Add quota reservations so concurrent deployments cannot both pass stale checks.
- Add a central audit helper and audit access, resource, quota, secret, job, login, and destructive changes.
- Implement full project update/archive; never hard-delete a project with history or resources.
- Encrypt secrets with an externally supplied master key; support create, rotate, and archive without returning decrypted values.
- Redact secret values from API errors, job logs, audit metadata, and runner output.
- Refuse live infrastructure operations if secret encryption is not configured correctly.

Suggested addition: `quota_reservations` with scope, job/deployment, dimensions, expiry, and committed/released state.

Acceptance:

1. Concurrent fake deployments cannot exceed a project quota.
2. Project archive preserves resources, assignments, jobs, and audit history.
3. Test secrets are encrypted at rest and absent from API/log output.
4. Audit history identifies who changed access or requested work.

### Slice 2 — Proxmox Service and Read-Only Inventory

Branch: `classroom/02-proxmox-inventory`

Purpose: prove connectivity and actual-state modeling before mutations.

Scope:

- Add an application-owned `proxmox.Service` interface, typed DTOs, fake implementation, and real adapter.
- Support health, node, storage, network/SDN, guest/template, and guest-status/config reads.
- Add admin APIs/UI for connection health and inventory sync.
- Reconcile inventory without automatically taking ownership of discovered objects.
- Link managed resources only through durable Organesson tags plus cluster/VMID identity.
- Record in-sync, missing, changed, unmanaged, ambiguous, and error drift states.
- Never mutate or delete during discovery.
- Document an environment-gated real-cluster smoke test.

The service boundary should resemble:

```go
type Service interface {
    Health(ctx context.Context) error
    ListNodes(ctx context.Context) ([]Node, error)
    ListStorages(ctx context.Context) ([]Storage, error)
    ListNetworks(ctx context.Context) ([]Network, error)
    ListGuests(ctx context.Context) ([]Guest, error)
    GetGuest(ctx context.Context, node string, vmID int) (Guest, error)
}
```

Acceptance:

1. Fake-driven sync has complete tests.
2. A test cluster displays nodes, templates, networks, and guests.
3. Unmanaged objects remain untouched.
4. Missing managed guests become drifted rather than erasing history.

### Slice 3 — Jobs and Operations UI

Branch: `classroom/03-jobs-operations`

Purpose: make long-running actions observable and recoverable.

Scope:

- Add scoped job list/detail/log APIs and an operations UI.
- Show status, progress, requester, target, attempts, timestamps, safe errors, and logs.
- Add cancellation request/confirmation semantics and transient-versus-permanent retry classification.
- Persist immutable job input, operation type, attempt count, error code, and safe summary.
- Add heartbeat/lease and startup recovery for abandoned running jobs.
- Add API idempotency keys and worker operation keys.
- Add configurable global/per-node concurrency.
- Polling for UI log/status updates is sufficient for the first classroom release.
- Drain active tasks for a bounded time during scheduler shutdown.
- Retain user-visible job/audit history while pruning scheduler internals by policy.

Acceptance:

1. A fake multi-step job exposes progress/logs and supports cancellation.
2. Duplicate API requests do not duplicate work.
3. Restart recovery produces a safe, documented result.
4. Students cannot inspect other groups’ jobs or logs.

### Slice 4 — VM Power and Browser Console

Branch: `classroom/04-vm-console`

Purpose: deliver the first student-useful live workflow.

Scope:

- Extend the Proxmox service with start, shutdown/stop, reboot, task polling, and console-ticket methods.
- Queue power mutations and enforce the corresponding resource permission.
- Revalidate managed identity, project ownership, tags, and authorization in the worker.
- Display actual power state and its freshness.
- Broker short-lived browser console sessions/WebSockets without exposing the durable Proxmox token.
- Validate user session, origin, target, and expiry when the console connects.
- Rate-limit console tickets and power actions.
- Add pending UI states and fake tests for failure, timeout, duplication, forbidden access, lost resources, and expired tickets.

Acceptance:

1. Assigned students can see, start, stop, reboot, and console a test VM.
2. Unassigned students cannot discover or operate it.
3. Every action creates a job and audit event.
4. No durable Proxmox credential reaches the browser.

Milestone A: staff can manually create/tag lab VMs and use Organesson for assignment, console, and power.

### Slice 5 — Lab Blueprints and Network Allocation

Branch: `classroom/05-lab-blueprints`

Purpose: represent `goal.md` as repeatable desired state.

Scope:

- Add blueprint, immutable blueprint-version, deployment, deployment-resource, network-pool, and allocation models.
- Model eight VMs, templates, sizing, NICs, networks, boot ordering, and configuration roles.
- Model WAN/LAN/DMZ, exercise VLAN/management parameters, IPv4/IPv6, DNS/domain settings, and optional inbound web access.
- Support student groups of one to N without changing ownership.
- Preflight total quotas and known cluster capacity.
- Allocate VMIDs, names, VLAN/VXLAN/VNet IDs, WAN addresses, and external ports transactionally.
- Validate templates, NICs, CIDRs, pool capacity, and cluster capabilities.
- Add instructor blueprint/version, validation, and deployment-preview UI.
- Provide a reference `cyber-lab-v1` blueprint matching `goal.md`.

Recommended names:

```text
deployment: it666-fa26-g03
VMs:        it666-fa26-g03-fw, -sw-lan, -sw-dmz, -ad, -win11, -fedora, -web, -ubuntu
networks:   it666-fa26-g03-wan, -lan, -dmz
tag:        organesson.deployment=<deployment UUID>
```

Acceptance:

1. Previewing three groups shows eight VMs and complete allocations for each.
2. Allocations cannot collide; quota/pool/capacity failure occurs before queuing.
3. Editing a used blueprint creates a new version without changing deployments.

### Slice 6 — Sandboxed Runner Foundation

Branch: `classroom/06-runner-foundation`

Purpose: safely execute Terraform/OpenTofu and Ansible through jobs.

Scope:

- Add separate runner interfaces for Terraform/OpenTofu and Ansible.
- Use explicit arguments, clean environments, private bounded workdirs, timeouts, cancellation, and output limits; do not invoke a shell for derived values.
- Keep one workdir/state lock per deployment/run and store durable state/artifacts outside the web root.
- Persist plan summaries, state references, run records, and sanitized output.
- Stream redacted stdout/stderr into job logs.
- Allow only configured source roots and immutable module/playbook revisions; never arbitrary user code/paths.
- Add runner health/version checks.
- Run plans automatically if authorized; require explicit authorization for apply/destroy.
- Test with fake executables for success, failure, malformed output, timeout, cancellation, process-group cleanup, path escape, and redaction.

Acceptance:

1. Fake Terraform plan/apply and Ansible jobs run through the scheduler and UI.
2. Cancellation terminates the child process group.
3. Secrets are absent from stored logs/output.
4. Paths cannot escape configured sources/workdirs.

### Slice 7 — Terraform/OpenTofu Lab Provisioning

Branch: `classroom/07-lab-provisioning`

Purpose: create and destroy the complete virtual topology.

Scope:

- Add a pinned root module for the reference lab.
- Clone templates, connect NICs to allocated WAN/LAN/DMZ networks, and tag every object.
- Return structured VMIDs, placement, MACs, addresses, and network IDs.
- Validate outputs before importing local resource/VM/network rows.
- Implement plan, apply, refresh/reconcile, and destroy jobs.
- Make operations idempotent/resumable where possible and provide safe recovery otherwise.
- Respect network/appliance/server/client boot dependencies.
- Add bulk deployment with bounded concurrency and per-group status.
- Release quota reservations on pre-creation failure; retain them for partial infrastructure until reconciled.
- Never auto-destroy partial deployments. Expose retry, reconcile, and confirmed destroy.
- Show desired/actual resources, outputs, drift, and jobs in deployment details.

Acceptance:

1. One test deployment creates eight correctly wired VMs and isolated networks.
2. A second deployment uses distinct allocation identities and cannot cross LAN/DMZ boundaries.
3. Reapply is stable; partial failure is visible and recoverable.
4. Destroy removes only correctly tagged deployment objects and then releases allocations.

### Slice 8 — Ansible Baseline and Exercise Preparation

Branch: `classroom/08-lab-configuration`

Purpose: produce a consistent starting lab without solving student exercises.

Scope:

- Generate per-deployment inventory from validated Terraform outputs and live facts.
- Add connection strategies for Linux, Windows/WinRM, pfSense, and ArubaOS-CX; explicitly report unsupported automation.
- Add first-boot readiness probes and retries.
- Implement versioned baseline, collect-facts, exercise-reset, and connectivity-validation workflows.
- Configure only the starting state, credentials, hostnames, and minimum management path; do not apply graded target state.
- Store structured per-host results and show a redacted inventory preview.
- Validate expected guests, NIC/MAC mappings, and management reachability before marking ready.

The baseline must prepare pfSense WAN/LAN/DMZ, Aruba management/uplinks, AD and Windows, the pinned Fedora starting release, DMZ hosts, and IPv4/IPv6 exercise parameters.

Acceptance:

1. A new lab enters `ready` only after validation.
2. Baseline reruns are idempotent.
3. An unreachable host yields an actionable partial result.
4. Graded target configuration is not pre-applied.

Milestone B: the complete unconfigured/exercise-ready topology can be repeatedly deployed and configured.

### Slice 9 — Roster, Bulk Assignment, and Student UX

Branch: `classroom/09-student-experience`

Purpose: make class setup and daily use practical.

Scope:

- Add a course setup flow: org/project, roster import, group formation, blueprint selection, quota preview, and deployment.
- Support CSV plus existing LDAP/FreeIPA lookup with stable matching/duplicate handling.
- Bulk-create groups and review membership before provisioning.
- Automatically create and assign each deployment asset group after provisioning.
- Create/select a least-privilege student role with required read, power, and console permissions.
- Add a “My Lab” page with topology, status, console/power controls, instructor notes, and maintenance state.
- Hide administrative infrastructure fields from students.
- Add safe instructor bulk controls with target review, concurrency, and per-lab results.
- Add access explanations for why a user can access a resource.
- Validate responsive/accessibility behavior for core pages and console launch.

Acceptance:

1. Import a sample roster with individual and multi-student groups.
2. Deploy/attach and assign labs in bulk.
3. Each student sees exactly the intended lab; TAs see project labs according to role.
4. Membership changes update access without transferring resource ownership.

### Slice 10 — Grading, Reset, and Course Lifecycle

Branch: `classroom/10-grading-reset`

Purpose: cover the required exercises and repeated classroom operation.

Scope:

- Add versioned grading workflows mapped to blueprint versions.
- Run read-mostly grading against one, selected, or all labs with bounded concurrency.
- Store structured checks, points, feedback, version, timestamps, and evidence references.
- Separate student feedback from instructor diagnostics.
- Implement robust checks for:
  - Device/interface discovery.
  - Firewall policy and exposed services.
  - VLANs and segmentation.
  - Management access only from the intended Fedora VM.
  - Windows joined to the expected AD domain.
  - Fedora upgraded to the configured target release.
  - DMZ web server reachable through the intended external path.
  - Intended LAN/DMZ flows and blocked flows.
  - DMZ IPv6 addressing, routing, and filtering.
- Prefer behavioral checks over brittle configuration-string matching.
- Implement reset via safe playbook, approved snapshot, or full destroy/redeploy.
- Archive courses by disabling access before optional delayed destruction.
- Require confirmation for bulk reset/destroy and export results as CSV/JSON.

Acceptance:

1. Known-good and intentionally broken labs produce expected results.
2. Students cannot inspect instructor-only grading logic/evidence.
3. Reset restores the documented start state.
4. Redeploy preserves historical jobs, audits, and grade attempts.
5. Archive removes access without immediately deleting evidence.

Milestone C: all non-honors tasks in `goal.md` can be prepared, attempted, graded, and reset.

### Slice 11 — Pilot Hardening

Branch: `classroom/11-pilot-hardening`

Purpose: make the feature-complete workflow dependable for a supervised class.

Scope:

- Rehearse at 1, 5, and full expected class size.
- Measure clone/configure duration, API pressure, storage growth, job throughput, and console concurrency.
- Tune concurrency, retries, timeouts, and start staggering.
- Add health/status for database, scheduler, runners, Proxmox, storage, pools, and templates.
- Add monitoring hooks for stuck jobs, low capacity, pool exhaustion, drift, and repeated failures.
- Document and test backup/restore for databases, encryption key, runner state/artifacts, blueprint sources, and grades.
- Test recovery from app/scheduler restarts, Proxmox timeout, partial apply, lost Ansible connectivity, and restore.
- Add retention for console sessions, logs, artifacts, states, grade evidence, and audits.
- Review CSRF/origin protections, rate limits, cookies, CSP, dependencies, destructive routes, and tenant boundaries.
- Freeze blueprint, templates, providers, playbooks, and OS targets before class.

Required runbooks:

- Install/configure the app and encryption key.
- Connect/validate Proxmox and register templates.
- Configure network/address pools.
- Create a course, import roster, and deploy labs.
- Diagnose, retry, reset, archive, and destroy.
- Move a student between groups.
- Recover pool exhaustion or partial provisioning.
- Restore backups.
- Emergency-disable all student operations without deleting labs.

Acceptance:

1. Full-class rehearsal completes within the acceptable deployment window.
2. Automated cross-lab isolation passes.
3. Representative concurrent consoles and power operations remain bounded.
4. Restart/recovery and clean-instance restore succeed.
5. A scripted instructor-to-student-to-grading-to-reset walkthrough succeeds.
6. No critical authorization, secret-exposure, isolation, or destruction defects remain.

Milestone D: recommended classroom-ready cutoff.

### Slice 12 — Optional Honors Attack Automation

Branch: `classroom/12-honors-attacks`

Purpose: support optional adversarial exercises without threatening shared infrastructure.

Scope:

- Use dedicated disposable attacker infrastructure.
- Target explicit deployment UUIDs/addresses, never arbitrary CIDRs.
- Enforce rate, duration, packet/connection, and concurrency ceilings.
- Network-block access to management, Proxmox, Organesson, storage, LDAP, and other labs.
- Add controlled scanning, seeded-account authentication attempts, bounded HTTP load, and safe exploit validation.
- Do not run volumetric DDoS on shared infrastructure; simulate pressure within operator-approved limits.
- Audit authorization, targets, limits, results, and emergency stops.
- Add a kill switch that cancels work and disables the attacker path.

Acceptance:

1. Attacks reach only the selected lab and obey limits after worker restart.
2. The kill switch stops work and blocks traffic.
3. Other labs and management services remain unaffected.

This slice is optional and must not delay the normal course.

## 6. Required Data Model

Retain and extend existing organizations, projects, groups, RBAC, resources, asset groups/assignments, Proxmox inventory, quotas, jobs, audit, and secrets.

Add:

- `QuotaReservation`.
- `LabBlueprint` and immutable `LabBlueprintVersion`.
- `LabDeployment` and `LabDeploymentResource`.
- `NetworkPool` and `NetworkAllocation`.
- `TerraformWorkspace`/`TerraformRun` or equivalent state records.
- `AnsibleInventory` and `AnsibleRun`.
- `GradeWorkflow`, `GradeAttempt`, and `GradeCheckResult`.
- `ConsoleSession` with short expiry.

Recommended deployment states:

```text
draft -> validated -> queued -> planning -> provisioning -> configuring -> ready
ready -> degraded | resetting | destroying | archived
any active operation -> error
destroying -> destroyed
```

Validate transitions centrally and audit them. API clients must not write arbitrary lifecycle state.

## 7. Suggested API Surface

Names are illustrative and should follow repository conventions.

```text
GET/POST/PATCH       /api/v1/quota-policies
GET/POST/DELETE      /api/v1/projects/:id/quota-bindings

GET/POST             /api/v1/system/proxmox/clusters
POST                 /api/v1/system/proxmox/clusters/:id/sync
GET                  /api/v1/system/proxmox/clusters/:id/inventory

GET                  /api/v1/jobs
GET                  /api/v1/jobs/:id
GET                  /api/v1/jobs/:id/logs
POST                 /api/v1/jobs/:id/cancel

POST                 /api/v1/resources/:id/actions/start
POST                 /api/v1/resources/:id/actions/stop
POST                 /api/v1/resources/:id/actions/reboot
POST                 /api/v1/resources/:id/console-sessions

GET/POST             /api/v1/lab-blueprints
GET/POST             /api/v1/lab-blueprints/:id/versions
POST                 /api/v1/lab-blueprints/:id/validate

GET/POST             /api/v1/projects/:id/lab-deployments
GET                  /api/v1/lab-deployments/:id
POST                 /api/v1/lab-deployments/:id/plan
POST                 /api/v1/lab-deployments/:id/apply
POST                 /api/v1/lab-deployments/:id/configure
POST                 /api/v1/lab-deployments/:id/grade
POST                 /api/v1/lab-deployments/:id/reset
POST                 /api/v1/lab-deployments/:id/destroy
POST                 /api/v1/lab-deployments/:id/reconcile
```

Bulk endpoints must accept explicit deployment IDs and return a parent job with child jobs. Never use a list filter as an implicit destructive target selector.

## 8. Testing and Security Gates

Automated coverage must include:

- Database validation, lifecycle transitions, allocation locking, quotas, and archive behavior.
- RBAC for students, groups, instructors, TAs, admins, and unrelated users on every endpoint.
- Shared contracts for fake/real Proxmox adapters where practical.
- Job/runner idempotency, cancellation, retries, restart recovery, timeouts, ordering, and redaction.
- Browser workflows for setup, assignment, student visibility, jobs, console, grading, and destruction.
- Terraform validation/fixture plans and Ansible syntax/fixture tests.

Required environment-gated tests:

1. Proxmox read-only health/inventory.
2. One small guest clone, power, console, and destroy.
3. Full eight-VM deployment.
4. Two-lab IPv4/IPv6 cross-isolation.
5. Baseline configuration and idempotent rerun.
6. Known-good/known-bad grading fixtures.
7. Full-class capacity rehearsal.

Security tests must prove:

- Assignment never transfers ownership.
- Students cannot enumerate other labs through IDs, lists, jobs, logs, grading, or consoles.
- Delegated admins cannot assign permissions they lack.
- Workers reject mismatched job/project/deployment/resource identity.
- Destroy cannot target untagged or ambiguously tagged objects.
- Secrets never appear in JSON, logs, errors, audit metadata, or stored command lines.
- Labs cannot reach each other’s LAN, DMZ, management interfaces, or console sessions.

## 9. Operations Recommendations

- Start with one app instance and embedded scheduler; add distributed workers only when justified.
- Run jobs as a dedicated OS user with access only to needed binaries/artifacts.
- Keep Terraform state/artifacts on backed-up storage, not ephemeral workdirs.
- Use a dedicated least-privilege Proxmox token plus application ownership guards.
- Separate application/Proxmox management, lab WAN, lab internals, and attacker infrastructure at the network layer.
- Maintain a non-production Proxmox target for release validation.
- Pre-warm templates and deploy before class instead of cloning everything at student login.
- Maintain spare network allocations and storage headroom.
- Archive first and destroy after a reviewed retention window.

## 10. Decisions Required Before Slice 5

Slices 0–4 can proceed now. Blueprint/provisioning work requires recorded answers for:

- Cluster, nodes, storage, SDN/bridge model, and VLAN/VXLAN capabilities.
- Reused-isolated versus unique LAN/DMZ private prefixes.
- WAN IPv4/IPv6 pools, routing, DNS, NAT, and inbound web exposure.
- Template IDs, versions, licensing, initialization, and resource sizing.
- ArubaOS-CX and pfSense automation methods.
- AD domain, Windows licensing/activation, and reset method.
- Frozen Fedora starting/target versions. Do not resolve “latest” dynamically.
- Expected class/group sizes, capacity, and acceptable deployment time.
- Grading points, feedback/evidence retention, and rerun policy.
- Snapshot versus redeploy reset policy.

Record answers in versioned operator and blueprint configuration, not source constants.

## 11. Definition of Done

The goal is decently classroom ready when:

- A course roster becomes one deployment per group without manual database changes.
- Each deployment creates the expected eight VMs and WAN/LAN/DMZ wiring.
- Two simultaneous labs pass automated IPv4/IPv6 cross-isolation tests.
- Students can see, power, and console only assigned resources.
- Staff can configure, grade, reset, redeploy, archive, and explicitly destroy through queued audited jobs.
- Every required `goal.md` task has known-good and known-bad grading fixtures.
- Quota, allocation, template, capacity, and cluster failures surface before destructive work.
- Partial failures/restarts do not create untracked duplicate infrastructure or unsafe destruction.
- Secrets are protected and no critical authorization/isolation defect is open.
- Full-class rehearsal, backup/restore, and operator runbooks are complete.

Perfection is not required. Appliance-specific documented recovery, polling-based job updates, supervised operations, and a single app instance are acceptable for the first course run.
