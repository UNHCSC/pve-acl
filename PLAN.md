# Proxmox Cloud Manager Charter

## 1. Purpose

Organesson Cloud is a self-service cloud control plane for managing shared virtualization infrastructure backed by Proxmox. Its goal is to provide AWS-like organization, ownership, quotas, permissions, automation, and repeatable deployment workflows while preserving raw Proxmox access for trusted infrastructure administrators.

The system exists to support:

- Lab-wide infrastructure administration
- Cybersecurity club infrastructure
- Competition team environments
- Teaching and training lab environments represented as org subtrees
- Per-student and per-group VM/CT ownership
- Bulk provisioning of repeatable lab topologies
- Terraform/OpenTofu and Ansible-backed deployments

## 2. Design Philosophy

Proxmox remains the hypervisor and infrastructure backend.

Organesson Cloud owns the higher-level cloud concepts:

- Organizations
- Projects
- Groups
- Memberships
- Roles
- Quotas
- Ownership
- Templates
- Automation jobs
- Audit history

The application should not attempt to replace Proxmox for low-level administration. Instead, it should provide a safer, multi-tenant interface for normal users while allowing lab administrators to use raw Proxmox directly when needed.

## 3. Core Goals

### 3.1 Multi-Tenant Organization

The system must support a deep organization tree. Organizations are the only tenant hierarchy primitive: each organization may contain projects and any number of child organizations. Use child organizations to model clubs, teams, classes, semesters, sections, student cohorts, or any other nested administrative boundary.

- Lab administration
- Cybersecurity club
- Competition team
- Teaching programs
- Semester or cohort instances
- Student groups
- Project groups

Example hierarchy only:

```text
Lab
├── Admins
├── Club
│   ├── Officers
│   └── Members
├── Competition
│   ├── Coaches
│   └── Members
└── Teaching
    └── IT666-Fall2026
        ├── Instructors
        ├── TAs
        ├── Students
        └── Groups
```

Access control must follow this tree. A role binding on an organization applies to that organization, all descendant organizations, and projects attached anywhere below it. A project-scoped binding applies only to that project unless a higher org binding grants the same permission through inheritance.

### 3.2 Ownership

Every managed resource should have a clear owner.

Owners may be:

* A user
* A group
* A project
* An organization

Examples only:

```text
VM 1201 is owned by user alice
VM 1301 is owned by IT666-Fall2026 Group 03
Network it666-lab1-g03 is owned by IT666-Fall2026 Group 03
Template ubuntu-24.04-base is owned by Lab Admins
```

### 3.3 Role-Based Access Control

The system must support local roles and permissions. Roles are first-class, editable collections of permission grants. The application should ship with a small set of system roles needed to bootstrap the environment, but administrators must be able to create custom roles, add or remove permissions from those roles, and bind any number of roles to users or groups over the appropriate scope.

The MVP access model should stay intentionally small:

* Groups answer "who is in this set?"
* Roles answer "what permissions does this grant?"
* Role bindings answer "who receives which role, and where?"

Group membership roles such as `member`, `manager`, and `owner` are only for administering that group itself. They should not be treated as infrastructure permissions. Infrastructure permissions should come from role bindings so there is one primary access path to reason about.

Example roles only:

* LabAdmin
* OrgAdmin
* Instructor
* TeachingAssistant
* ClubOfficer
* CompetitionCoach
* Student
* GroupMember
* Viewer

Admin permissions supersede lower-level memberships.

Example only:

```text
A user may be both:
- LabAdmin
- IT666-Fall2026 TeachingAssistant

The LabAdmin role should grant full system-level authority regardless of lower org role.
```

### 3.4 Resource Quotas

The system must support resource limits at multiple levels:

* User quota
* Group quota
* Project quota
* Organization quota

Quota-controlled resources should include:

* vCPU count
* RAM
* Disk/storage
* Number of VMs
* Number of containers
* Number of networks
* Network bandwidth policy
* Public IP assignments, if applicable

### 3.5 Bulk Provisioning

The system must support bulk deployment workflows.

Examples only:

* Create one VM per student
* Create one isolated lab network per student
* Create a multi-VM topology per group
* Deploy an entire org/project lab from a template
* Destroy or archive all resources for a completed lab

### 3.6 VM/CT Management

Users should be able to manage resources they are permitted to access.

Supported VM/CT operations should include:

* View
* Start
* Stop
* Reboot
* Console
* Snapshot
* Clone
* Resize, if allowed
* Reconfigure CPU/RAM/disk/network, if allowed
* Delete, if allowed

### 3.7 Virtual Network Management

The system should support virtual network creation and assignment.

Network models may include:

* Shared org/project network
* Per-student isolated network
* Per-group isolated network
* Competition team network
* Admin infrastructure network
* Internet-restricted network
* Internal-only network

### 3.8 Terraform/OpenTofu Support

The system should be able to generate, store, and execute Terraform/OpenTofu deployments.

Terraform/OpenTofu support should include:

* Template-based generation
* Workspace/state tracking
* Plan/apply/destroy jobs
* Output capture
* Logs
* Approval gates, if needed

### 3.9 Ansible Support

The system should support Ansible automation.

Ansible support should include:

* Inventory generation
* Playbook execution
* Per-resource or per-lab runs
* Logs
* Result status
* SSH credential handling
* Post-provision configuration workflows

### 3.10 Console Access

The system should support browser-based VM/CT console access.

Console access must be permission-checked through the application before connecting to Proxmox.

### 3.11 Auditability

The system must keep an audit trail of important actions.

Examples:

* User login
* VM created
* VM deleted
* VM console opened
* VM reconfigured
* Quota changed
* Role assigned
* Terraform applied
* Ansible playbook run
* Network created
* Template changed

## 4. Non-Goals

The system is not intended to:

* Replace Proxmox for trusted lab administrators
* Become a full public cloud
* Support arbitrary untrusted internet users
* Hide all Proxmox concepts from administrators
* Reimplement every Proxmox feature immediately
* Require Kubernetes or OpenStack for the MVP

## 5. Recommended System Architecture

```text
Browser UI
  ↓
Go/Fiber Web App
  ↓
Application Services
  ├── Auth Service
  ├── RBAC Service
  ├── Quota Service
  ├── Proxmox Service
  ├── Network Service
  ├── Terraform/OpenTofu Service
  ├── Ansible Service
  └── Audit Service
  ↓
Job Queue
  ↓
Workers
  ├── Proxmox Worker
  ├── Terraform/OpenTofu Worker
  └── Ansible Worker
  ↓
Proxmox Cluster
```

## 6. Recommended Database Structure

The database should store cloud-level state. Proxmox remains the source of truth for actual VM execution state, but the application should maintain ownership, intent, permissions, quota, and audit data.

The schema blocks below are planning references for the data model. Application code should not use hand-written direct SQL strings for normal database operations; it should use the Go `gomysql` library and its query/build/execution APIs so database access stays consistent, reviewable, and reusable.

Identity and access data is application-owned. Initial setup should create the main lab organization, the configured administrator group or groups, and the built-in administrator role. It should not import every LDAP group into the application. LDAP groups should enter the cloud access model only when they are configured as bootstrap admin groups or when an administrator creates a cloud group and explicitly marks its membership as synced from LDAP. Local-only groups remain entirely managed inside the application.

## 7. Core Tables

### 7.1 users

Stores local and external users.

```sql
users
-----
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
username            VARCHAR(128) UNIQUE NOT NULL
display_name        VARCHAR(255)
email               VARCHAR(255)
auth_source         ENUM('local', 'ldap', 'oidc') NOT NULL
external_id         VARCHAR(255)
is_active           BOOLEAN NOT NULL DEFAULT TRUE
is_system_admin     BOOLEAN NOT NULL DEFAULT FALSE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 7.2 groups

Represents local groups, LDAP-synced groups, staff groups, teams, officer groups, project cohorts, and other collections that can be used as ACL subjects. A group is a collection of people today and should be able to grow into a collection of people, assets, projects, and other application-managed subjects as the platform matures.

```sql
groups
------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
name                VARCHAR(255) NOT NULL
slug                VARCHAR(255) UNIQUE NOT NULL
description         TEXT
group_type          ENUM('admin', 'club', 'competition', 'student_group', 'project', 'custom') NOT NULL
parent_group_id     BIGINT NULL
sync_source         ENUM('local', 'ldap') NOT NULL DEFAULT 'local'
external_id         VARCHAR(255)
sync_membership     BOOLEAN NOT NULL DEFAULT FALSE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

Example groups only:

```text
admins
club
club-officers
competition
competition-coaches
teaching
it666-fall2026-staff
it666-fall2026-students
it666-fall2026-group-03
```

LDAP import policy:

* During initial setup, create/sync only the configured administrator groups needed to bootstrap site access.
* Do not import all LDAP groups by default during login.
* Additional LDAP-backed groups should be created in the application with `sync_source = 'ldap'`, `external_id` set to the LDAP group name, and `sync_membership = TRUE`.
* Local-only groups should use `sync_source = 'local'` and should be managed entirely through application membership tools.

### 7.3 group_memberships

Maps users into groups. The `membership_role` field is for group administration only: `manager` and `owner` can maintain group membership, but resource and project permissions still come from role bindings.

```sql
group_memberships
-----------------
id                  BIGINT PRIMARY KEY
user_id             BIGINT NOT NULL
group_id            BIGINT NOT NULL
membership_role     ENUM('member', 'manager', 'owner') NOT NULL DEFAULT 'member'
created_at          DATETIME NOT NULL

UNIQUE(user_id, group_id)
```

### 7.4 roles

Defines reusable permission roles. System roles are seeded by the application and protected from accidental removal. Custom roles are administrator-managed permission bundles and can be bound to any number of users or groups.

```sql
roles
-----
id                  BIGINT PRIMARY KEY
name                VARCHAR(128) UNIQUE NOT NULL
description         TEXT
is_system_role      BOOLEAN NOT NULL DEFAULT FALSE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

Example roles:

```text
LabAdmin
OrgAdmin
Instructor
TeachingAssistant
Student
GroupMember
VMViewer
VMOperator
VMOwner
NetworkManager
TemplateManager
AutomationRunner
```

### 7.5 permissions

Defines atomic permissions.

```sql
permissions
-----------
id                  BIGINT PRIMARY KEY
name                VARCHAR(128) UNIQUE NOT NULL
description         TEXT
```

Example permissions:

```text
vm.read
vm.create
vm.start
vm.stop
vm.reboot
vm.console
vm.snapshot
vm.clone
vm.resize
vm.reconfigure
vm.delete

ct.read
ct.create
ct.start
ct.stop
ct.console
ct.delete

network.read
network.create
network.update
network.delete
network.attach

template.read
template.create
template.update
template.delete
template.clone

terraform.plan
terraform.apply
terraform.destroy

ansible.run

quota.read
quota.update

user.manage
group.manage
role.manage
audit.read
```

### 7.6 role_permissions

Maps roles to permission grants. Editing a custom role means adding and removing rows here; role bindings do not need to change when the permission bundle changes.

```sql
role_permissions
----------------
id                  BIGINT PRIMARY KEY
role_id             BIGINT NOT NULL
permission_id       BIGINT NOT NULL

UNIQUE(role_id, permission_id)
```

### 7.7 role_bindings

Assigns any number of roles to users or groups over a scope. In the UI these should be presented as "access grants" so an administrator can see the entire access surface in one place instead of hunting inside each group.

```sql
role_bindings
-------------
id                  BIGINT PRIMARY KEY
role_id             BIGINT NOT NULL

subject_type        ENUM('user', 'group') NOT NULL
subject_id          BIGINT NOT NULL

scope_type          ENUM('global', 'org', 'project', 'group', 'resource') NOT NULL
scope_id            BIGINT NULL

created_by_user_id  BIGINT
created_at          DATETIME NOT NULL
```

Example:

```text
Group admins has LabAdmin on global
Group it666-fall2026-staff has TeachingAssistant on org /Teaching/IT666-Fall2026
Group it666-fall2026-group-03 has GroupMember on project it666-lab1-group-03
```

## 8. Tenant and Project Tables

Aside from the initial system setup step that creates the primary lab organization and its initial administrator group, organizations, sub-organizations, projects, and their related groups should be normal application-managed entities. Users with the appropriate OrgAdmin or delegated admin permissions must be able to create, modify, archive, and reorganize them at any time without requiring a database migration or manual operator intervention.

### 8.1 organizations

Logical tenant tree nodes. Organizations may be roots or children of another organization. Projects attach directly to an organization; teaching programs, semesters, clubs, teams, and cohorts are all modeled as organization nodes rather than separate structural tables.

```sql
organizations
-------------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
name                VARCHAR(255) NOT NULL
slug                VARCHAR(255) UNIQUE NOT NULL
description         TEXT
parent_org_id       BIGINT NULL
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

Example organizations only:

```text
lab-admin
club
competition
teaching
teaching/it666-fall2026
```

### 8.2 projects

A project is the primary ownership and isolation boundary.

```sql
projects
--------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
organization_id     BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
slug                VARCHAR(255) UNIQUE NOT NULL
project_type        ENUM('admin', 'club', 'competition', 'student', 'group', 'lab', 'custom') NOT NULL
description         TEXT
is_active           BOOLEAN NOT NULL DEFAULT TRUE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

Example projects only:

```text
it666-fall2026-alice
it666-fall2026-group-03
it666-fall2026-lab1
competition-practice-blue-team
club-web-infra
```

### 8.4 project_memberships

Maps users/groups to projects. For the MVP this table can remain a project-local convenience layer for showing collaborators and simple project roles. Long-term infrastructure permissions should converge on scoped role bindings so project membership does not become a second, conflicting RBAC system.

```sql
project_memberships
-------------------
id                  BIGINT PRIMARY KEY
project_id          BIGINT NOT NULL
subject_type        ENUM('user', 'group') NOT NULL
subject_id          BIGINT NOT NULL
project_role        ENUM('viewer', 'operator', 'developer', 'manager', 'owner') NOT NULL
created_at          DATETIME NOT NULL

UNIQUE(project_id, subject_type, subject_id)
```

## 9. Resource Tables

### 9.1 resources

Generic base table for all managed resources.

```sql
resources
---------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
project_id          BIGINT NOT NULL
owner_type          ENUM('user', 'group', 'project', 'organization') NOT NULL
owner_id            BIGINT NOT NULL
resource_type       ENUM('vm', 'ct', 'network', 'template', 'volume', 'secret', 'terraform_workspace', 'ansible_inventory') NOT NULL
name                VARCHAR(255) NOT NULL
slug                VARCHAR(255)
status              ENUM('creating', 'ready', 'updating', 'deleting', 'deleted', 'error', 'unknown') NOT NULL
created_by_user_id  BIGINT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
deleted_at          DATETIME NULL
```

### 9.2 proxmox_clusters

Stores Proxmox cluster definitions.

```sql
proxmox_clusters
----------------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
name                VARCHAR(255) NOT NULL
api_url             VARCHAR(512) NOT NULL
verify_tls          BOOLEAN NOT NULL DEFAULT TRUE
credential_secret_id BIGINT
is_active           BOOLEAN NOT NULL DEFAULT TRUE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 9.3 proxmox_nodes

Stores known Proxmox nodes.

```sql
proxmox_nodes
-------------
id                  BIGINT PRIMARY KEY
cluster_id          BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
status              VARCHAR(64)
cpu_total           INT
memory_total_mb     BIGINT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL

UNIQUE(cluster_id, name)
```

### 9.4 virtual_machines

Cloud-managed QEMU VMs.

```sql
virtual_machines
----------------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
cluster_id          BIGINT NOT NULL
node_id             BIGINT
proxmox_vmid        INT NOT NULL
name                VARCHAR(255) NOT NULL
cpu_cores           INT NOT NULL
memory_mb           BIGINT NOT NULL
disk_gb             BIGINT
os_type             VARCHAR(128)
template_id         BIGINT NULL
power_state         ENUM('running', 'stopped', 'paused', 'unknown') NOT NULL DEFAULT 'unknown'
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL

UNIQUE(cluster_id, proxmox_vmid)
```

### 9.5 containers

Cloud-managed LXC containers.

```sql
containers
----------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
cluster_id          BIGINT NOT NULL
node_id             BIGINT
proxmox_vmid        INT NOT NULL
name                VARCHAR(255) NOT NULL
cpu_cores           INT NOT NULL
memory_mb           BIGINT NOT NULL
disk_gb             BIGINT
template_id         BIGINT NULL
power_state         ENUM('running', 'stopped', 'unknown') NOT NULL DEFAULT 'unknown'
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL

UNIQUE(cluster_id, proxmox_vmid)
```

### 9.6 virtual_networks

Managed virtual networks.

```sql
virtual_networks
----------------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
cluster_id          BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
network_type        ENUM('bridge', 'vlan', 'vxlan', 'sdn_zone', 'isolated', 'routed', 'nat') NOT NULL
cidr_ipv4           VARCHAR(64)
cidr_ipv6           VARCHAR(128)
vlan_id             INT NULL
vxlan_id            INT NULL
gateway_ipv4        VARCHAR(64)
gateway_ipv6        VARCHAR(128)
is_internet_routable BOOLEAN NOT NULL DEFAULT FALSE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 9.7 resource_network_interfaces

Maps VMs/CTs to networks.

```sql
resource_network_interfaces
---------------------------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT NOT NULL
network_id          BIGINT NOT NULL
mac_address         VARCHAR(32)
ipv4_address        VARCHAR(64)
ipv6_address        VARCHAR(128)
interface_name      VARCHAR(64)
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 9.8 templates

Tracks VM/CT templates.

```sql
templates
---------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
cluster_id          BIGINT NOT NULL
template_type       ENUM('vm', 'ct', 'terraform', 'ansible', 'lab_blueprint') NOT NULL
name                VARCHAR(255) NOT NULL
version             VARCHAR(64)
description         TEXT
source_resource_id  BIGINT NULL
is_public           BOOLEAN NOT NULL DEFAULT FALSE
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

## 10. Quota Tables

### 10.1 quota_policies

Reusable quota policies.

```sql
quota_policies
--------------
id                  BIGINT PRIMARY KEY
name                VARCHAR(255) NOT NULL
description         TEXT
max_vms             INT
max_containers      INT
max_vcpu            INT
max_memory_mb       BIGINT
max_storage_gb      BIGINT
max_networks        INT
max_public_ips      INT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 10.2 quota_bindings

Applies quota policies to users, groups, organizations, or projects.

```sql
quota_bindings
--------------
id                  BIGINT PRIMARY KEY
quota_policy_id     BIGINT NOT NULL
subject_type        ENUM('user', 'group', 'organization', 'project') NOT NULL
subject_id          BIGINT NOT NULL
created_at          DATETIME NOT NULL

UNIQUE(subject_type, subject_id)
```

### 10.3 resource_usage_snapshots

Optional table for cached usage calculations.

```sql
resource_usage_snapshots
------------------------
id                  BIGINT PRIMARY KEY
subject_type        ENUM('user', 'group', 'organization', 'project') NOT NULL
subject_id          BIGINT NOT NULL
vm_count            INT NOT NULL DEFAULT 0
container_count     INT NOT NULL DEFAULT 0
vcpu_used           INT NOT NULL DEFAULT 0
memory_mb_used      BIGINT NOT NULL DEFAULT 0
storage_gb_used     BIGINT NOT NULL DEFAULT 0
network_count       INT NOT NULL DEFAULT 0
created_at          DATETIME NOT NULL
```

## 11. Automation Tables

### 11.1 jobs

Generic async job tracking.

```sql
jobs
----
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
job_type            ENUM('proxmox', 'terraform', 'ansible', 'lab_deploy', 'lab_destroy', 'sync', 'cleanup') NOT NULL
status              ENUM('queued', 'running', 'succeeded', 'failed', 'cancelled') NOT NULL
requested_by_user_id BIGINT
project_id          BIGINT NULL
resource_id         BIGINT NULL
queue_id            VARCHAR(255)
started_at          DATETIME NULL
finished_at         DATETIME NULL
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 11.2 job_logs

Stores job output.

```sql
job_logs
--------
id                  BIGINT PRIMARY KEY
job_id              BIGINT NOT NULL
stream              ENUM('stdout', 'stderr', 'system') NOT NULL
message             TEXT NOT NULL
created_at          DATETIME NOT NULL
```

### 11.3 terraform_workspaces

Tracks Terraform/OpenTofu state.

```sql
terraform_workspaces
--------------------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
project_id          BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
working_dir         VARCHAR(1024)
state_backend       ENUM('local', 's3', 'http', 'other') NOT NULL
status              ENUM('new', 'planned', 'applied', 'destroyed', 'error') NOT NULL
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 11.4 terraform_runs

Tracks individual plan/apply/destroy runs.

```sql
terraform_runs
--------------
id                  BIGINT PRIMARY KEY
workspace_id        BIGINT NOT NULL
job_id              BIGINT NOT NULL
action              ENUM('init', 'plan', 'apply', 'destroy') NOT NULL
status              ENUM('queued', 'running', 'succeeded', 'failed', 'cancelled') NOT NULL
plan_output         LONGTEXT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 11.5 ansible_inventories

Stores generated or uploaded inventories.

```sql
ansible_inventories
-------------------
id                  BIGINT PRIMARY KEY
resource_id         BIGINT UNIQUE NOT NULL
project_id          BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
inventory_content   LONGTEXT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 11.6 ansible_runs

Tracks playbook executions.

```sql
ansible_runs
------------
id                  BIGINT PRIMARY KEY
project_id          BIGINT NOT NULL
inventory_id        BIGINT
job_id              BIGINT NOT NULL
playbook_name       VARCHAR(255) NOT NULL
status              ENUM('queued', 'running', 'succeeded', 'failed', 'cancelled') NOT NULL
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

## 12. Lab Blueprint Tables

### 12.1 lab_blueprints

Defines reusable lab deployments.

```sql
lab_blueprints
--------------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
name                VARCHAR(255) NOT NULL
description         TEXT
version             VARCHAR(64)
blueprint_format    ENUM('native', 'terraform', 'ansible', 'mixed') NOT NULL
created_by_user_id  BIGINT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 12.2 lab_deployments

Represents an instantiated lab.

```sql
lab_deployments
---------------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
blueprint_id        BIGINT NOT NULL
project_id          BIGINT NOT NULL
name                VARCHAR(255) NOT NULL
status              ENUM('creating', 'ready', 'updating', 'destroying', 'destroyed', 'error') NOT NULL
created_by_user_id  BIGINT
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

### 12.3 lab_deployment_targets

Tracks who/what a lab was deployed for.

```sql
lab_deployment_targets
----------------------
id                  BIGINT PRIMARY KEY
deployment_id       BIGINT NOT NULL
target_type         ENUM('user', 'group', 'project') NOT NULL
target_id           BIGINT NOT NULL
created_at          DATETIME NOT NULL
```

## 13. Secrets Tables

### 13.1 secrets

Stores references to encrypted secrets.

```sql
secrets
-------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
name                VARCHAR(255) NOT NULL
secret_type         ENUM('proxmox_token', 'ssh_key', 'password', 'api_token', 'terraform_var', 'ansible_var') NOT NULL
encrypted_value     BLOB NOT NULL
owner_type          ENUM('system', 'user', 'group', 'project') NOT NULL
owner_id            BIGINT NULL
created_at          DATETIME NOT NULL
updated_at          DATETIME NOT NULL
```

## 14. Audit Tables

### 14.1 audit_events

Stores security and activity logs.

```sql
audit_events
------------
id                  BIGINT PRIMARY KEY
uuid                CHAR(36) UNIQUE NOT NULL
actor_user_id       BIGINT NULL
action              VARCHAR(255) NOT NULL
target_type         VARCHAR(128)
target_id           BIGINT NULL
project_id          BIGINT NULL
source_ip           VARCHAR(128)
user_agent          TEXT
metadata_json       JSON
created_at          DATETIME NOT NULL
```

Example actions:

```text
auth.login
auth.logout
vm.create
vm.delete
vm.start
vm.stop
vm.console.open
network.create
network.delete
quota.update
role.assign
terraform.apply
ansible.run
lab.deploy
lab.destroy
```

## 15. Recommended MVP Database Scope

For the first implementation, build only these tables:

```text
users
groups
group_memberships
roles
permissions
role_permissions
role_bindings

organizations
projects
project_memberships

resources
proxmox_clusters
proxmox_nodes
virtual_machines
containers
virtual_networks

quota_policies
quota_bindings

jobs
job_logs
audit_events
secrets
```

Add Terraform, Ansible, and lab blueprint tables after VM/CT provisioning and RBAC are stable.

## 16. Recommended Permission Model

Use this shape internally:

```text
subject: user or group
action: permission string
object: scoped resource path
```

Example objects:

```text
/global
/org/teaching
/org/teaching/it666-fall2026
/project/it666-fall2026-group-03
/resource/vm/1201
/resource/network/it666-lab1-g03
```

Example checks:

```text
Can Evan create VMs in IT666-Fall2026?
subject = user:evan
action = vm.create
object = /org/teaching/it666-fall2026

Can Alice open console for VM 1201?
subject = user:alice
action = vm.console
object = /resource/vm/1201

Can Group 03 manage its own network?
subject = group:it666-fall2026-group-03
action = network.update
object = /project/it666-fall2026-group-03
```

## 17. Recommended Resource Lifecycle

### VM Creation

```text
1. User submits VM creation request
2. API checks authentication
3. API checks permission: vm.create
4. API checks quota
5. API creates resource row with status=creating
6. API queues provisioning job
7. Worker creates VM in Proxmox
8. Worker tags VM in Proxmox
9. Worker updates virtual_machines row
10. Worker updates resource status=ready
11. Audit event is written
```

### VM Deletion

```text
1. User requests delete
2. API checks permission: vm.delete
3. API marks resource status=deleting
4. API queues deletion job
5. Worker destroys VM in Proxmox
6. Worker marks resource deleted
7. Audit event is written
```

## 18. Recommended Proxmox Integration Rules

The application should tag all managed Proxmox resources.

Recommended tags:

```text
managed-by:lab-cloud
project:<project-slug>
org:<org-path-or-slug>
owner-type:<user|group|project>
owner:<owner-slug>
```

Example:

```text
managed-by:lab-cloud
project:it666-fall2026-group-03
org:teaching/it666-fall2026
owner-type:group
owner:it666-fall2026-group-03
```

The app should periodically sync with Proxmox to detect drift.

Drift examples:

```text
VM deleted manually in Proxmox
VM renamed manually in Proxmox
VM moved to another node
VM CPU/RAM changed outside the app
VM power state changed
```

## 19. Recommended MVP Milestones

### Milestone 1: Identity and RBAC

* Login
* Users
* Groups
* Group memberships
* Roles
* Permissions
* Role bindings surfaced as access grants

### Milestone 2: Proxmox Inventory

* Add Proxmox cluster
* Discover nodes
* Discover existing VMs/CTs
* Display VM/CT state
* Tag app-managed resources

### Milestone 3: Basic VM Lifecycle

* Create VM from template
* Start/stop/reboot
* Delete
* View console
* Audit events

### Milestone 4: Projects and Quotas

* Projects
* Project membership
* Quota policies
* Quota enforcement
* Per-project resource views

### Milestone 5: Organization Tree Support

* Organization and sub-organization creation
* Instructor/teaching-assistant/student groups as normal groups
* Bulk student import
* Per-student VM creation
* Per-group VM creation

### Milestone 6: Networks

* Create virtual networks
* Attach VM/CT to networks
* Per-student/per-group isolation

### Milestone 7: Automation

* Job queue
* Job logs
* Terraform/OpenTofu runner
* Ansible runner

### Milestone 8: Lab Blueprints

* Define reusable labs
* Deploy lab per student
* Deploy lab per group
* Destroy/archive labs

## 20. Implementation Strategy

Implementation should proceed in small vertical slices that can be prompted, implemented, reviewed, and tested independently. Each part should leave the app compiling, preserve the existing Go/Fiber package layout, and use `gomysql` registrations, filters, migrations, and table helpers instead of hand-written direct SQL.

Each implementation prompt should usually ask for one part below, include the target files, and require at least `go test ./...` before completion. If client files change, also run the client production build from `client/`.

Current repo shape:

```text
main.go                 app startup
config/                 TOML config loading
auth/                   LDAP authentication, JWT sessions, test user injection
db/                     gomysql models, registration, migrations, helper queries
app/                    Fiber routes, page handlers, API handlers
client/views/           small Fiber templates that mount React roots
client/src/             React/TypeScript components and shared client helpers built by Vite
```

### 20.1 Current Progress

Completed or substantially implemented:

* Temporary-test-database setup for Go tests.
* Auth route tests, middleware tests, enum tests, config tests, and focused `db` helper tests.
* Authenticated API middleware that resolves the current auth user and local database user.
* MVP gomysql models for users, groups, memberships, organizations, projects, project memberships, roles, permissions, role permissions, role bindings, quotas, resources, Proxmox inventory, jobs, audit events, and secrets.
* Idempotent initial setup for the root lab organization, configured administrator groups, `LabAdmin`, core permissions, and admin role bindings.
* LDAP login sync limited to bootstrap admin groups and explicitly configured LDAP-synced cloud groups. Arbitrary LDAP groups are no longer imported by default.
* Central RBAC evaluation through roles, permissions, role bindings, scopes, and site-admin override behavior.
* User, group, role, permission, role-binding/access-grant, and project helper APIs.
* Project create/list/detail/delete, project membership management, project member role editing, direct user lookup/sync, and group assignment to projects.
* Custom role creation and role permission grant editing.
* Access UI organized around the simplified model: groups are membership sets, roles are permission bundles, and access grants are role bindings over scopes.
* Dashboard tabs for overview, directory, people, identity, and access, with account menu, toasts, URL routing, and user-scoped recents.
* React/TypeScript client split into reusable components, views, API helpers, and tree utilities, built with Bun, Vite, and Tailwind.

Known deliberate leftovers:

* Some legacy local ACL tables and routes still exist for compatibility while the MVP cloud model takes over. Remove them only after replacement routes and tests cover the same use cases.
* Organization management is now the tenant hierarchy foundation; dedicated org CRUD/archive screens still need polish.
* Resources, quotas, Proxmox integration, and job execution are scaffolded in the schema but not yet functional product workflows.
* Audit events exist and setup writes some audit rows, but audit logging is not yet consistently applied to all management actions.

Current baseline checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.2 Working Rules Going Forward

* Keep the access model simple: groups answer "who", roles answer "what", access grants answer "where".
* Do not add a second permission system in project membership or group membership. Project/group-local roles may control local administration, but infrastructure permissions should come from scoped role bindings.
* Prefer one vertical slice at a time: schema/helper, API, UI, tests, build.
* Keep handlers thin. Put repeated gomysql lookups and mutations into `db` helpers or small app helper functions.
* Use readable labels in APIs wherever the UI would otherwise need to render raw IDs.
* Do not remove legacy tables or routes in the same patch as unrelated feature work.

### 20.3 Next Slice: Access Polish and Delegation

Goal: finish the identity/access foundation before moving into resources.

Implementation scope:

* Add edit/delete endpoints for roles where safe. System roles should not be deletable from the UI.
* Add group update/archive endpoints and a cleaner group detail payload.
* Add optional delegated grant management: group owners may manage access grants only within their own group scope, while global grants remain site-admin only.
* Add audit events for user, group, role, permission grant, and access grant changes.
* Add tests for system role protection, group update/archive behavior, and delegated scoped grant management.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.4 Next Slice: Organization Tree

Goal: make the tenant hierarchy real enough to attach projects, groups, quotas, and scopes to it.

Implementation scope:

* Add organization CRUD and archive behavior.
* Add sub-organization create/move/archive behavior.
* Add UI screens or focused panels for organizations only after API tests exist.
* Allow group and project creation to select an organization.
* Update access grants so org/project scopes can be selected by readable name rather than raw ID.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.5 Next Slice: Project Cleanup and Quota Policies

Goal: make projects the useful operational boundary before introducing real resources.

Implementation scope:

* Add project update/archive instead of hard delete as the default UI behavior.
* Add project-level access grant shortcuts that create normal role bindings rather than separate permission logic.
* Add quota policy CRUD and quota bindings for projects and groups.
* Add usage-calculation helpers that can run before Proxmox integration.
* Add tests for project archive, quota binding, and quota calculation with local resource rows.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.6 Next Slice: Resource Registry Without Proxmox Mutations

Goal: create a cloud-level inventory and permission surface before issuing infrastructure changes.

Implementation scope:

* Implement resource CRUD/archive APIs for VM, CT, network, template, volume, secret, Terraform workspace, and Ansible inventory rows.
* Attach resources to projects and owners.
* Enforce access grants and quota checks before resource creation/update.
* Add dashboard project detail sections for resources.
* Add audit events for create, update, archive, and assignment changes.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.7 Next Slice: Proxmox Service Boundary

Goal: isolate Proxmox access behind a testable service before any mutating VM action exists.

Implementation scope:

* Add a Proxmox service package with an interface and fake implementation.
* Wrap the configured Proxmox client in that service.
* Implement cluster/node discovery and read-only VM/CT inventory sync first.
* Keep handlers talking to the service interface, not directly to the Proxmox client.
* Add tests with the fake service.

Acceptance checks:

```text
go test ./...
```

### 20.8 Next Slice: Jobs and Audit Trail

Goal: prepare for long-running VM, network, Terraform/OpenTofu, and Ansible workflows.

Implementation scope:

* Implement job creation, status transitions, logs, cancellation semantics, and in-process development workers.
* Write audit events through a helper so handlers and workers do not duplicate audit formatting.
* Add job detail APIs and UI surfaces for queued/running/completed work.
* Add tests for job state transitions, job logs, cancellation, and audit writes.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.9 Next Slice: VM/CT Lifecycle

Goal: ship the first useful end-to-end Proxmox-backed workflow.

Implementation scope:

* Implement VM create-from-template, start, stop, reboot, delete/archive, and console-ticket workflows first.
* Repeat the same shape for containers after VM behavior is stable.
* Queue mutating operations as jobs and update resource status as jobs progress.
* Enforce access grants and quotas before job creation.
* Use a non-production Proxmox cluster or fake service until the workflow is proven.

Acceptance checks:

```text
go test ./...
cd client && bun run check && bun run build
```

### 20.10 Later Slices: Automation and Lab Blueprints

Goal: add Terraform/OpenTofu, Ansible, and lab blueprints only after inventory, RBAC, quotas, jobs, and VM/CT lifecycle are stable.

Implementation scope:

* Add Terraform/OpenTofu workspace and run tables.
* Add Ansible inventory and run tables.
* Add lab blueprint and deployment tables.
* Implement each runner through the job queue with logs, approval gates where needed, and project-scoped permissions.
* Use temporary working directories and fake commands in tests before enabling real infrastructure execution.

Acceptance checks:

```text
go test ./...
```

### 20.11 Likely Go Packages to Add

* Queuing: `github.com/hibiken/asynq`, `github.com/redis/go-redis/v9`
* Proxmox API: `github.com/luthermonson/go-proxmox`
* ACL policy engine, only if the custom gomysql RBAC path becomes too costly: `github.com/casbin/casbin/v2`
* Terraform/OpenTofu: `github.com/hashicorp/hcl/v2`, `github.com/hashicorp/terraform-exec/tfexec`
* Typing help for Terraform/OpenTofu values: `github.com/zclconf/go-cty/cty`

## 21. Long-Term Features

Potential future features:

* Approval workflows
* Budget-like quota dashboards
* Scheduled lab teardown
* VM expiration dates
* Snapshot policies
* Backup policies
* Per-org/project templates
* Cloud-init customization
* DNS integration
* IPAM integration
* NetBox integration
* FreeIPA group sync
* OIDC login
* Grafana dashboards
* Per-project audit exports
* Student-facing lab instructions
