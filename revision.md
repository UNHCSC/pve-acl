There still seems to be some desync between project/org permissions and global things. Here's what I NEED you to fix NOW.

1. There may only be ONE root level org. You cannot add multiple root level orgs. Everything must be a child of that.
2. Remove the access tab entirely! I believe it's creating too much confusion. Then remove backend for it. (Make a backup though, parts of it will be brought back in directory)
3. The distinction between orgs and projects should be this: "An org may have children orgs and projects, but may not have resources. A project may not have any children orgs or projects, but may have resources"
4. Here's how I want groups and roles and memberships to work (rest of this document):

- User: a human user synced by a provider (ldap, etc.)
- User Group: a group of users.
- Role: a set of permissions that can be assigned to users or groups.
- Membership: a user or group can be a member of an org or project with a specific set of roles granted to them by that org or project through direct assignment from their user or groups, or through inheritance from parent orgs.
- Asset Group: a group of assets that can be assigned to users or groups. This is for things like VMs, where you want to assign it to a specific user or group of users, but you still want it to be owned by a project for billing and organizational purposes.

## Notes (Desires)

- The concept of "you get X through Y" where X is either an asset or a role, and Y is either a project or org. This is the core of the access control system. It should be clear to users how they have access to something, and it should be clear to admins how to grant access to something.
    - Example: "You get the permission to audit the IT666 project since you are a member of the IT666 Students group, which is a member of the IT666 project, which grants that permission to its members."
- Additionally, assets have this extra ability to be assigned directly to a user, rather than a project/group in a project. However, the asset is still owned by a project, and the user only gets access to it through that project. This is for things like VMs, where you want to assign it to a specific user or group of users, but you still want it to be owned by a project for billing and organizational purposes.
    - Example: "You get vm.console access to VM-XXX since the VM is assigned to you, managed by the IT666 Project."
    - Plain English Use-case: As the TA in IT666, I would like to assign a VM to each student for them to use for their assignments. I want students to only be able to console/power their VM, and not be able to do that to other students' VMs. However, I want the VMs to be owned by the IT666 project, for organizational purposes. Furthermore, they should all reside in a "Student VMs" asset group, so that I can easily see all the student VMs and manage them at once in bulk.
    - Plain English Use-case-2: As the TA in IT666, I am assigning a project to the students which does group work. Each group will have a user group within the IT666 project. Then, I will give a few VMs to each group, by creating a "Group N VMs" asset group, and linking the user and asset groups together. This way, I can easily manage the VMs for each group, and the students will get access to the VMs through their group membership.

## Access Control Example

### Tree

There is exactly one root organization. Everything else hangs from it.

- Lab (Root Org)
    - Courses (Org)
        - IT666 (Project)
        - CS527 (Project)
    - Club (Org)
        - NECCDC Training (Project)
    - Evan's Research (Project)

Rules shown by this tree:

- `Lab` is the only root-level organization.
- `Courses` and `Club` may contain child orgs or projects.
- `IT666`, `CS527`, `NECCDC Training`, and `Evan's Research` may contain resources, asset groups, role grants, and memberships.
- Projects may not contain child orgs or child projects.
- Orgs may not directly own resources.

### People

- Alice (Systems Administrator)
- Bob (Professor, teaching IT666)
- Charlie (Student, enrolled in IT666)
- Diana (Student, enrolled in IT666)
- Evan (TA, assisting with IT666, Systems Administrator)
- Gloria (Student, enrolled in IT666, member of club)
- Hannah (Student, member of club)

### User Groups

User groups are groups of people. A group has an owner scope, which is the org or project where it is managed. System-managed groups can only be maintained by system administrators.

- Admins (system-managed group): Alice, Evan
- Courses Instructors (owned by Courses org): Bob
- Courses Students (owned by Courses org): Charlie, Diana, Gloria
- Club Members (owned by Club org): Gloria, Hannah
- IT666 Instructors (owned by IT666 project): Bob
- IT666 Students (owned by IT666 project): Charlie, Diana, Gloria
- IT666 TAs (owned by IT666 project): Evan
- IT666 Group 01 (owned by IT666 project): Charlie, Diana

Important group-management rules:

- A project or org admin can manage local-only groups owned by their project or org.
- A project or org admin cannot toggle LDAP import/sync for a group unless they are also a system administrator.
- LDAP-synced groups are still user groups, but their membership source is system-managed.

### Roles

Roles are permission bundles. A role also has an owner scope, which controls who can edit it and where it can be assigned.

- Admin (system-managed role): all permissions.
- Courses Instructor (owned by Courses org): can create and manage course projects, course-owned groups, course-scoped roles, and resources inside descendant projects.
- Courses Student (owned by Courses org): can view course projects they are a member of and use student-facing resources granted to them.
- Club Officer (owned by Club org): can manage club projects, club-owned groups, and club training resources.
- IT666 Project Instructor (owned by IT666 project): can manage IT666 memberships, groups, roles, resources, quotas, and asset groups.
- IT666 Project TA (owned by IT666 project): can create student/group resources, assign IT666-owned user groups to asset groups, and operate student lab resources.
- IT666 Project Student (owned by IT666 project): can view IT666 and use assets assigned directly to them or to one of their IT666 groups.
- IT666 VM User (owned by IT666 project): can read, start, stop, reboot, and console into specifically assigned VMs.

Privilege-ceiling rule:

- A delegated admin may only create or edit a role using permissions they already have at that scope.
- A delegated admin may only assign roles whose permissions they already have at that scope.
- Bob can create an `IT666 Lab Auditor` role if it only contains permissions Bob has in IT666.
- Bob cannot assign `Admin`, cannot grant `user.manage` globally, and cannot grant permissions outside IT666 unless he has those permissions at the wider scope.

### Memberships And Role Grants

Membership means "this user or group participates in this org/project, and this org/project grants these roles to that member."

System/root memberships:

- Admins is a member of Lab with Admin.

Org memberships:

- Courses Instructors is a member of Courses with Courses Instructor.
- Courses Students is a member of Courses with Courses Student.
- Club Members is a member of Club with a Club Member role.

Project memberships:

- IT666 Instructors is a member of IT666 with IT666 Project Instructor.
- IT666 TAs is a member of IT666 with IT666 Project TA.
- IT666 Students is a member of IT666 with IT666 Project Student.
- IT666 Group 01 is a member of IT666 with IT666 Project Student.

Direct user memberships are allowed when they are clearer than making another group:

- Bob can be directly added to IT666 with IT666 Project Instructor.
- Evan can be directly added to Evan's Research with a project owner role, while still getting Admin through the Admins group.

Inheritance:

- A role granted by `Lab` applies to `Lab`, all child orgs, and all descendant projects.
- A role granted by `Courses` applies to `Courses`, `IT666`, `CS527`, and any future descendant org/project.
- A role granted by `IT666` applies only to IT666 and its resources.
- A role granted by `Club` does not apply to `Courses` or `IT666`.

### Asset Groups And Asset Assignments

Assets are owned by projects for organization, quota, billing, and lifecycle. Assets may also be assigned to users or user groups for access.

IT666 owns these resources:

- Asset Group: IT666 Student VMs
    - VM `it666-charlie-01`, assigned to Charlie with IT666 VM User.
    - VM `it666-diana-01`, assigned to Diana with IT666 VM User.
    - VM `it666-gloria-01`, assigned to Gloria with IT666 VM User.
- Asset Group: IT666 Group 01 VMs
    - VM `it666-g01-router`, assigned to IT666 Group 01 with IT666 VM User.
    - VM `it666-g01-workstation`, assigned to IT666 Group 01 with IT666 VM User.

Assignment rules:

- Assigning a VM to Charlie does not make Charlie the owner of the VM.
- IT666 remains the project owner for quota, billing, audit, and lifecycle.
- Charlie receives only the permissions attached to that assignment.
- TAs and instructors can still manage the VM through their IT666 project roles.
- Asset groups allow bulk management without changing project ownership.

### Access Explanations

The UI should be able to explain both the permission and the path that caused it.

Alice can manage all IT666 resources:

```text
Alice gets resource.manage on IT666 because:
Alice is in Admins.
Admins is a member of Lab with Admin.
Lab is an ancestor of Courses/IT666.
Admin includes resource.manage.
```

Bob can manage IT666 memberships and resources:

```text
Bob gets project.manage and group.manage on IT666 because:
Bob is in IT666 Instructors.
IT666 Instructors is a member of IT666 with IT666 Project Instructor.
IT666 Project Instructor includes project.manage and group.manage.
```

Charlie can console only Charlie's assigned VM:

```text
Charlie gets vm.console on it666-charlie-01 because:
it666-charlie-01 is owned by IT666.
it666-charlie-01 is assigned to Charlie with IT666 VM User.
IT666 VM User includes vm.console.
```

Charlie cannot console Diana's VM:

```text
Charlie does not get vm.console on it666-diana-01 because:
it666-diana-01 is assigned to Diana.
Charlie is not Diana.
Charlie does not have a project role that grants vm.console over all IT666 student VMs.
```

Diana can console the Group 01 VMs:

```text
Diana gets vm.console on it666-g01-router because:
Diana is in IT666 Group 01.
it666-g01-router is assigned to IT666 Group 01 with IT666 VM User.
IT666 VM User includes vm.console.
```

Gloria has unrelated permissions from two branches:

```text
Gloria gets student access in IT666 because she is in IT666 Students.
Gloria gets club access in NECCDC Training because she is in Club Members.
These paths do not merge into broader access outside their scopes.
```

Evan can act as a system administrator even though he is also a TA:

```text
Evan gets Admin because he is in Admins.
Evan also gets IT666 Project TA because he is in IT666 TAs.
Admin is stronger, but the UI should still show both paths when explaining access.
```

### What Admins Can Grant

Alice can:

- Create orgs and projects anywhere below Lab.
- Create system-managed roles and groups.
- Import or sync LDAP-backed groups.
- Assign Admin or any other role anywhere.

Bob can:

- Manage IT666 memberships if he has IT666 Project Instructor.
- Add users or user groups to IT666.
- Create IT666-scoped roles using only permissions Bob already has in IT666.
- Assign IT666-scoped roles to IT666 users or groups.
- Manage local-only IT666 groups, such as IT666 Group 01.

Bob cannot:

- Create another root org.
- Assign Admin.
- Grant permissions he does not have.
- Manage CS527 unless Courses or CS527 grants him access.
- Toggle LDAP import/sync for an IT666 group unless he is also a system administrator.

Evan can:

- Manage IT666 as a TA through IT666.
- Manage everything as a system administrator through Admins.
- The system should prefer showing the least surprising path for normal actions, but should expose all access paths in an explanation view.

### What The UI Should Feel Like

There should not be a global Access tab for normal access work.

- On an org page, admins manage child orgs, projects, org-owned groups, org-scoped roles, and org memberships.
- On a project page, admins manage project members, project-owned groups, project-scoped roles, resources, asset groups, and asset assignments.
- On a group page, admins manage local group membership and see where the group is a member.
- On a role page, admins manage role permissions and see where the role is assigned.
- A future "Access Overview" can exist for system administrators only, but it should be an inventory/deep-link surface, not the main workflow.

## Casbin Decision

This desired model is feasible.

Casbin will probably make the authorization checks easier and safer, as long as it is used as a policy engine rather than as the only source of truth.

Recommended direction:

- Keep the application database as the source of truth for orgs, projects, resources, users, user groups, roles, memberships, asset groups, assignments, audit history, and explanation paths.
- Add a second Casbin policy database or policy table set as a derived authorization index.
- Rebuild or incrementally sync Casbin policies from application-owned tables.
- Use Casbin for fast yes/no authorization checks.
- Use application-owned relationship tables for "why do I have this?" explanations.

Casbin model shape:

- Use RBAC with domains for scoped role grants.
- Treat org/project/resource scopes as domains.
- Add a custom domain matching function so an org grant applies to descendant orgs/projects/resources.
- Use resource/object matching for project-owned resources and assigned assets.
- Keep asset assignment facts in application tables; either mirror them into Casbin policies or pass resource attributes into a custom matcher.

Example policy concepts, not final syntax:

```text
p, role:admin, scope:/org/lab, *, allow
p, role:courses-instructor, scope:/org/lab/courses, project.manage, allow
p, role:it666-vm-user, scope:/project/it666, vm.console, allow

g, user:alice, group:admins
g, group:admins, role:admin, scope:/org/lab

g, user:bob, group:it666-instructors
g, group:it666-instructors, role:it666-project-instructor, scope:/project/it666

assign, asset:it666-charlie-01, user:charlie, role:it666-vm-user, scope:/project/it666
assign, asset:it666-g01-router, group:it666-group-01, role:it666-vm-user, scope:/project/it666
```

Casbin should not decide business lifecycle rules by itself:

- It should not decide whether an org may have resources.
- It should not decide whether a project may have children.
- It should not decide whether LDAP sync is allowed.
- It should not be the only place where membership and assignment history lives.

The application should still enforce those domain rules directly, then ask Casbin whether the current principal has the permission needed to perform the action.
