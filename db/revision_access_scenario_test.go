package db

import (
	"testing"
	"time"
)

func TestRevisionAccessControlScenario(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	lab := mustCreateScenarioOrganization(t, "Lab", "lab", nil)
	if _, err := CreateOrganization(OrganizationCreateInput{Name: "Shadow Root", Slug: "shadow-root"}); err == nil {
		t.Fatal("expected creating a second root organization to be rejected")
	} else {
		t.Logf("DENY  | system  | org.create      | Shadow Root              | exactly one root org is allowed: %v", err)
	}

	courses := mustCreateScenarioOrganization(t, "Courses", "courses", &lab.ID)
	club := mustCreateScenarioOrganization(t, "Club", "club", &lab.ID)
	it666 := mustCreateScenarioProject(t, "IT666", "it666", courses.ID)
	cs527 := mustCreateScenarioProject(t, "CS527", "cs527", courses.ID)
	neccdc := mustCreateScenarioProject(t, "NECCDC Training", "neccdc-training", club.ID)
	research := mustCreateScenarioProject(t, "Evan's Research", "evans-research", lab.ID)

	alice := mustCreateScenarioUser(t, "alice", "Alice")
	bob := mustCreateScenarioUser(t, "bob", "Bob")
	charlie := mustCreateScenarioUser(t, "charlie", "Charlie")
	diana := mustCreateScenarioUser(t, "diana", "Diana")
	evan := mustCreateScenarioUser(t, "evan", "Evan")
	gloria := mustCreateScenarioUser(t, "gloria", "Gloria")
	hannah := mustCreateScenarioUser(t, "hannah", "Hannah")

	admins := mustCreateScenarioCloudGroup(t, "Admins", "admins", GroupTypeAdmin, RoleBindingScopeGlobal, nil, CloudGroupSyncSourceLDAP, true, now)
	coursesInstructors := mustCreateScenarioCloudGroup(t, "Courses Instructors", "courses-instructors", GroupTypeCustom, RoleBindingScopeOrg, &courses.ID, CloudGroupSyncSourceLocal, false, now)
	coursesStudents := mustCreateScenarioCloudGroup(t, "Courses Students", "courses-students", GroupTypeStudentGroup, RoleBindingScopeOrg, &courses.ID, CloudGroupSyncSourceLocal, false, now)
	clubMembers := mustCreateScenarioCloudGroup(t, "Club Members", "club-members", GroupTypeClub, RoleBindingScopeOrg, &club.ID, CloudGroupSyncSourceLocal, false, now)
	it666Instructors := mustCreateScenarioCloudGroup(t, "IT666 Instructors", "it666-instructors", GroupTypeCustom, RoleBindingScopeProject, &it666.ID, CloudGroupSyncSourceLocal, false, now)
	it666Students := mustCreateScenarioCloudGroup(t, "IT666 Students", "it666-students", GroupTypeStudentGroup, RoleBindingScopeProject, &it666.ID, CloudGroupSyncSourceLocal, false, now)
	it666TAs := mustCreateScenarioCloudGroup(t, "IT666 TAs", "it666-tas", GroupTypeCustom, RoleBindingScopeProject, &it666.ID, CloudGroupSyncSourceLocal, false, now)
	it666Group01 := mustCreateScenarioCloudGroup(t, "IT666 Group 01", "it666-group-01", GroupTypeProject, RoleBindingScopeProject, &it666.ID, CloudGroupSyncSourceLocal, false, now)

	mustAddScenarioGroupMember(t, alice, admins)
	mustAddScenarioGroupMember(t, evan, admins)
	mustAddScenarioGroupMember(t, bob, coursesInstructors)
	mustAddScenarioGroupMember(t, charlie, coursesStudents)
	mustAddScenarioGroupMember(t, diana, coursesStudents)
	mustAddScenarioGroupMember(t, gloria, coursesStudents)
	mustAddScenarioGroupMember(t, gloria, clubMembers)
	mustAddScenarioGroupMember(t, hannah, clubMembers)
	mustAddScenarioGroupMember(t, bob, it666Instructors)
	mustAddScenarioGroupMember(t, charlie, it666Students)
	mustAddScenarioGroupMember(t, diana, it666Students)
	mustAddScenarioGroupMember(t, gloria, it666Students)
	mustAddScenarioGroupMember(t, evan, it666TAs)
	mustAddScenarioGroupMember(t, charlie, it666Group01)
	mustAddScenarioGroupMember(t, diana, it666Group01)

	adminRole := mustCreateScenarioRole(t, "Admin", "System administrator", true, RoleBindingScopeGlobal, nil, CorePermissions...)
	coursesInstructorRole := mustCreateScenarioRole(t, "Courses Instructor", "Manage course projects and resources", false, RoleBindingScopeOrg, &courses.ID,
		PermissionProjectManage,
		PermissionGroupManage,
		PermissionRoleManage,
		PermissionVMCreate,
		PermissionVMRead,
		PermissionVMConsole,
	)
	coursesStudentRole := mustCreateScenarioRole(t, "Courses Student", "View course resources", false, RoleBindingScopeOrg, &courses.ID, PermissionVMRead)
	clubMemberRole := mustCreateScenarioRole(t, "Club Member", "Use club training resources", false, RoleBindingScopeOrg, &club.ID, PermissionVMRead)
	it666InstructorRole := mustCreateScenarioRole(t, "IT666 Project Instructor", "Manage IT666 memberships, groups, roles, and resources", false, RoleBindingScopeProject, &it666.ID,
		PermissionProjectManage,
		PermissionGroupManage,
		PermissionRoleManage,
		PermissionVMCreate,
		PermissionVMRead,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
	)
	it666TARole := mustCreateScenarioRole(t, "IT666 Project TA", "Create and operate IT666 student resources", false, RoleBindingScopeProject, &it666.ID,
		PermissionVMCreate,
		PermissionVMRead,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
	)
	it666StudentRole := mustCreateScenarioRole(t, "IT666 Project Student", "View IT666 and use assigned assets", false, RoleBindingScopeProject, &it666.ID, PermissionVMRead)
	it666VMUserRole := mustCreateScenarioRole(t, "IT666 VM User", "Operate specifically assigned IT666 VMs", false, RoleBindingScopeProject, &it666.ID,
		PermissionVMRead,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
	)
	researchOwnerRole := mustCreateScenarioRole(t, "Research Project Owner", "Own Evan's research project", false, RoleBindingScopeProject, &research.ID,
		PermissionProjectManage,
		PermissionVMRead,
	)

	mustGrantScenarioOrgRole(t, lab, admins, adminRole)
	mustGrantScenarioOrgRole(t, courses, coursesInstructors, coursesInstructorRole)
	mustGrantScenarioOrgRole(t, courses, coursesStudents, coursesStudentRole)
	mustGrantScenarioOrgRole(t, club, clubMembers, clubMemberRole)
	mustGrantScenarioProjectRole(t, it666, it666Instructors, it666InstructorRole)
	mustGrantScenarioProjectRole(t, it666, it666TAs, it666TARole)
	mustGrantScenarioProjectRole(t, it666, it666Students, it666StudentRole)
	mustGrantScenarioProjectRole(t, it666, it666Group01, it666StudentRole)
	mustGrantScenarioUserProjectRole(t, research, evan, researchOwnerRole)

	studentVMs := mustCreateScenarioAssetGroup(t, it666.ID, "IT666 Student VMs", "it666-student-vms")
	group01VMs := mustCreateScenarioAssetGroup(t, it666.ID, "IT666 Group 01 VMs", "it666-group-01-vms")
	charlieVM := insertTestResource(t, it666.ID, "it666-charlie-01", now)
	dianaVM := insertTestResource(t, it666.ID, "it666-diana-01", now)
	gloriaVM := insertTestResource(t, it666.ID, "it666-gloria-01", now)
	routerVM := insertTestResource(t, it666.ID, "it666-g01-router", now)

	mustAttachScenarioResource(t, studentVMs, charlieVM)
	mustAttachScenarioResource(t, studentVMs, dianaVM)
	mustAttachScenarioResource(t, studentVMs, gloriaVM)
	mustAttachScenarioResource(t, group01VMs, routerVM)

	mustAssignScenarioResource(t, it666, charlieVM, RoleBindingSubjectUser, charlie.ID, it666VMUserRole)
	mustAssignScenarioResource(t, it666, dianaVM, RoleBindingSubjectUser, diana.ID, it666VMUserRole)
	mustAssignScenarioResource(t, it666, gloriaVM, RoleBindingSubjectUser, gloria.ID, it666VMUserRole)
	mustAssignScenarioAssetGroup(t, it666, group01VMs, RoleBindingSubjectGroup, it666Group01.ID, it666VMUserRole)

	workstationVM := insertTestResource(t, it666.ID, "it666-g01-workstation", now)
	mustAttachScenarioResource(t, group01VMs, workstationVM)

	logRevisionScenarioMap(t, lab, courses, club, it666, cs527, neccdc, research)
	logScenarioMembership(t, "Alice", alice, admins, "system administrator through LDAP-managed Admins")
	logScenarioMembership(t, "Bob", bob, it666Instructors, "project instructor path for IT666")
	logScenarioMembership(t, "Charlie", charlie, it666Group01, "group project assets path")
	logScenarioMembership(t, "Diana", diana, it666Group01, "group project assets path")
	logScenarioMembership(t, "Gloria", gloria, clubMembers, "separate Club branch path")
	logScenarioMembership(t, "Evan", evan, admins, "system administrator path")
	logScenarioMembership(t, "Evan", evan, it666TAs, "local TA path")
	t.Logf("LDAP  | %-7s | %-22s | sync_source=%s sync_membership=%t", "Admins", "system-managed group", admins.SyncSource, admins.SyncMembership)
	t.Logf("OWNER | %-24s | project=%s | direct assignment does not transfer VM ownership", charlieVM.Name, it666.Name)

	t.Log("EXPECTATIONS")
	t.Log("want  | got   | actor   | permission     | target                   | explanation")
	t.Log("------+-------+---------+----------------+--------------------------+------------------------------------------------------------")

	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Alice",
		User:        alice,
		Permission:  PermissionVMDelete,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &dianaVM.ID,
		Target:      dianaVM.Name,
		Want:        true,
		Explanation: "Admins is a Lab member with Admin; Lab is an ancestor of IT666",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Bob",
		User:        bob,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &it666.ID,
		Target:      it666.Name,
		Want:        true,
		Explanation: "IT666 Instructors is a project member with IT666 Project Instructor",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Bob",
		User:        bob,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &cs527.ID,
		Target:      cs527.Name,
		Want:        true,
		Explanation: "Courses Instructors is a Courses member; Courses is an ancestor of CS527",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Bob",
		User:        bob,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &neccdc.ID,
		Target:      neccdc.Name,
		Want:        false,
		Explanation: "Courses and IT666 grants do not cross into the Club branch",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Bob",
		User:        bob,
		Permission:  PermissionRoleManage,
		ScopeType:   RoleBindingScopeGlobal,
		ScopeID:     nil,
		Target:      "global",
		Want:        false,
		Explanation: "delegated course/project roles do not grant global role administration",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Charlie",
		User:        charlie,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &charlieVM.ID,
		Target:      charlieVM.Name,
		Want:        true,
		Explanation: "the VM is directly assigned to Charlie with IT666 VM User",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Charlie",
		User:        charlie,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &dianaVM.ID,
		Target:      dianaVM.Name,
		Want:        false,
		Explanation: "Diana's VM is assigned to Diana, not Charlie",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Charlie",
		User:        charlie,
		Permission:  PermissionVMDelete,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &charlieVM.ID,
		Target:      charlieVM.Name,
		Want:        false,
		Explanation: "IT666 VM User permits operation, not destructive lifecycle changes",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Diana",
		User:        diana,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &routerVM.ID,
		Target:      routerVM.Name,
		Want:        true,
		Explanation: "IT666 Group 01 is assigned the whole Group 01 asset group",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Diana",
		User:        diana,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &workstationVM.ID,
		Target:      workstationVM.Name,
		Want:        true,
		Explanation: "new resources inherit existing asset-group assignments",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Gloria",
		User:        gloria,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &gloriaVM.ID,
		Target:      gloriaVM.Name,
		Want:        true,
		Explanation: "the VM is directly assigned to Gloria with IT666 VM User",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Gloria",
		User:        gloria,
		Permission:  PermissionVMConsole,
		ScopeType:   RoleBindingScopeResource,
		ScopeID:     &routerVM.ID,
		Target:      routerVM.Name,
		Want:        false,
		Explanation: "Gloria is not in IT666 Group 01",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Gloria",
		User:        gloria,
		Permission:  PermissionVMRead,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &neccdc.ID,
		Target:      neccdc.Name,
		Want:        true,
		Explanation: "Club Members is a Club member; Club is an ancestor of NECCDC",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Gloria",
		User:        gloria,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &cs527.ID,
		Target:      cs527.Name,
		Want:        false,
		Explanation: "student and club paths do not combine into course project management",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Hannah",
		User:        hannah,
		Permission:  PermissionVMRead,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &neccdc.ID,
		Target:      neccdc.Name,
		Want:        true,
		Explanation: "Club Members grants Club-scoped training access",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Hannah",
		User:        hannah,
		Permission:  PermissionVMRead,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &it666.ID,
		Target:      it666.Name,
		Want:        false,
		Explanation: "Club membership does not flow into Courses",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Evan",
		User:        evan,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &neccdc.ID,
		Target:      neccdc.Name,
		Want:        true,
		Explanation: "Admins gives Evan the root Admin path",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Evan",
		User:        evan,
		Permission:  PermissionVMCreate,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &it666.ID,
		Target:      it666.Name,
		Want:        true,
		Explanation: "Evan also has the local IT666 TA path",
	})
	assertScenarioPermission(t, scenarioPermissionExpectation{
		Actor:       "Evan",
		User:        evan,
		Permission:  PermissionProjectManage,
		ScopeType:   RoleBindingScopeProject,
		ScopeID:     &research.ID,
		Target:      research.Name,
		Want:        true,
		Explanation: "direct user membership can own a project when clearer than a group",
	})
}

type scenarioPermissionExpectation struct {
	Actor       string
	User        *User
	Permission  PermissionKey
	ScopeType   RoleBindingScope
	ScopeID     *int
	Target      string
	Want        bool
	Explanation string
}

func assertScenarioPermission(t *testing.T, expectation scenarioPermissionExpectation) {
	t.Helper()

	groupIDs, err := CloudGroupIDsForUser(expectation.User.ID)
	if err != nil {
		t.Fatalf("CloudGroupIDsForUser(%s) returned error: %v", expectation.Actor, err)
	}
	got, err := HasPermission(PermissionCheck{
		UserID:     expectation.User.ID,
		GroupIDs:   groupIDs,
		Permission: expectation.Permission,
		ScopeType:  expectation.ScopeType,
		ScopeID:    expectation.ScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission(%s, %s, %s) returned error: %v", expectation.Actor, expectation.Permission.String(), expectation.Target, err)
	}

	wantLabel := scenarioAccessLabel(expectation.Want)
	gotLabel := scenarioAccessLabel(got)
	t.Logf("%-5s | %-5s | %-7s | %-14s | %-24s | %s", wantLabel, gotLabel, expectation.Actor, expectation.Permission.String(), expectation.Target, expectation.Explanation)
	if got != expectation.Want {
		t.Fatalf("%s %s on %s: expected %t, got %t", expectation.Actor, expectation.Permission.String(), expectation.Target, expectation.Want, got)
	}
}

func logRevisionScenarioMap(t *testing.T, lab, courses, club *Organization, it666, cs527, neccdc, research *Project) {
	t.Helper()
	t.Log(`
SCENARIO TREE
Lab (root org)
|-- Courses (org)
|   |-- IT666 (project)
|   |   |-- IT666 Student VMs (asset group)
|   |   |   |-- it666-charlie-01 -> Charlie
|   |   |   |-- it666-diana-01   -> Diana
|   |   |   |-- it666-gloria-01  -> Gloria
|   |   |-- IT666 Group 01 VMs (asset group)
|   |       |-- it666-g01-router      -> IT666 Group 01
|   |       |-- it666-g01-workstation -> IT666 Group 01
|   |-- CS527 (project)
|-- Club (org)
|   |-- NECCDC Training (project)
|-- Evan's Research (project)
`)
	t.Logf("IDS   | root=%d courses=%d club=%d it666=%d cs527=%d neccdc=%d research=%d", lab.ID, courses.ID, club.ID, it666.ID, cs527.ID, neccdc.ID, research.ID)
}

func logScenarioMembership(t *testing.T, actor string, user *User, group *CloudGroup, reason string) {
	t.Helper()

	if _, found, err := CloudGroupMembershipForUserAndGroup(user.ID, group.ID); err != nil {
		t.Fatalf("CloudGroupMembershipForUserAndGroup(%s, %s) returned error: %v", actor, group.Name, err)
	} else if !found {
		t.Fatalf("expected %s to belong to %s", actor, group.Name)
	}
	t.Logf("MEMBR | %-7s | %-22s | %s", actor, group.Name, reason)
}

func scenarioAccessLabel(allowed bool) string {
	if allowed {
		return "ALLOW"
	}
	return "DENY"
}

func mustCreateScenarioOrganization(t *testing.T, name, slug string, parentID *int) *Organization {
	t.Helper()

	org, err := CreateOrganization(OrganizationCreateInput{
		Name:        name,
		Slug:        slug,
		ParentOrgID: parentID,
	})
	if err != nil {
		t.Fatalf("CreateOrganization(%s) returned error: %v", name, err)
	}
	return org
}

func mustCreateScenarioProject(t *testing.T, name, slug string, orgID int) *Project {
	t.Helper()

	project, err := CreateProject(ProjectCreateInput{
		Name:           name,
		Slug:           slug,
		OrganizationID: orgID,
		ProjectType:    ProjectTypeLab,
	})
	if err != nil {
		t.Fatalf("CreateProject(%s) returned error: %v", name, err)
	}
	return project
}

func mustCreateScenarioUser(t *testing.T, username, displayName string) *User {
	t.Helper()

	user, _, err := EnsureUser(username, displayName, username+"@example.test", "test", username)
	if err != nil {
		t.Fatalf("EnsureUser(%s) returned error: %v", username, err)
	}
	return user
}

func mustCreateScenarioCloudGroup(t *testing.T, name, slug string, groupType GroupType, ownerScopeType RoleBindingScope, ownerScopeID *int, syncSource string, syncMembership bool, now time.Time) *CloudGroup {
	t.Helper()

	if syncSource == "" {
		syncSource = CloudGroupSyncSourceLocal
	}
	group := &CloudGroup{
		UUID:           slug + "-scenario-uuid",
		Name:           name,
		Slug:           slug,
		GroupType:      groupType,
		OwnerScopeType: ownerScopeType,
		OwnerScopeID:   scenarioCopyIntPointer(ownerScopeID),
		SyncSource:     syncSource,
		SyncMembership: syncMembership,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if group.OwnerScopeType == RoleBindingScopeGlobal {
		group.OwnerScopeID = nil
	}
	if err := CloudGroups.Insert(group); err != nil {
		t.Fatalf("insert scenario cloud group %s: %v", name, err)
	}
	return group
}

func mustAddScenarioGroupMember(t *testing.T, user *User, group *CloudGroup) {
	t.Helper()

	if _, err := EnsureCloudGroupMembership(user.ID, group.ID, MembershipRoleMember); err != nil {
		t.Fatalf("EnsureCloudGroupMembership(%s, %s) returned error: %v", user.Username, group.Name, err)
	}
}

func mustCreateScenarioRole(t *testing.T, name, description string, isSystemRole bool, ownerScopeType RoleBindingScope, ownerScopeID *int, permissions ...PermissionKey) *Role {
	t.Helper()

	role, err := CreateRole(RoleCreateInput{
		Name:           name,
		Description:    description,
		IsSystemRole:   isSystemRole,
		OwnerScopeType: ownerScopeType,
		OwnerScopeID:   scenarioCopyIntPointer(ownerScopeID),
	})
	if err != nil {
		t.Fatalf("CreateRole(%s) returned error: %v", name, err)
	}
	for _, key := range permissions {
		permission, _, err := ensurePermission(key.String())
		if err != nil {
			t.Fatalf("ensurePermission(%s) returned error: %v", key.String(), err)
		}
		if _, err := EnsureRolePermission(role.ID, permission.ID); err != nil {
			t.Fatalf("EnsureRolePermission(%s, %s) returned error: %v", role.Name, key.String(), err)
		}
	}
	return role
}

func mustGrantScenarioOrgRole(t *testing.T, org *Organization, group *CloudGroup, role *Role) {
	t.Helper()

	if _, err := EnsureOrganizationMembership(org.ID, ProjectMemberSubjectGroup, group.ID, MembershipRoleMember); err != nil {
		t.Fatalf("EnsureOrganizationMembership(%s, %s) returned error: %v", org.Name, group.Name, err)
	}
	if err := EnsureOrganizationMemberRoleBinding(org.ID, ProjectMemberSubjectGroup, group.ID, role.ID); err != nil {
		t.Fatalf("EnsureOrganizationMemberRoleBinding(%s, %s, %s) returned error: %v", org.Name, group.Name, role.Name, err)
	}
}

func mustGrantScenarioProjectRole(t *testing.T, project *Project, group *CloudGroup, role *Role) {
	t.Helper()

	if _, err := EnsureProjectMembership(project.ID, ProjectMemberSubjectGroup, group.ID); err != nil {
		t.Fatalf("EnsureProjectMembership(%s, %s) returned error: %v", project.Name, group.Name, err)
	}
	if err := EnsureProjectMemberRoleBinding(project.ID, ProjectMemberSubjectGroup, group.ID, role.ID); err != nil {
		t.Fatalf("EnsureProjectMemberRoleBinding(%s, %s, %s) returned error: %v", project.Name, group.Name, role.Name, err)
	}
}

func mustGrantScenarioUserProjectRole(t *testing.T, project *Project, user *User, role *Role) {
	t.Helper()

	if _, err := EnsureProjectMembership(project.ID, ProjectMemberSubjectUser, user.ID); err != nil {
		t.Fatalf("EnsureProjectMembership(%s, %s) returned error: %v", project.Name, user.Username, err)
	}
	if err := EnsureProjectMemberRoleBinding(project.ID, ProjectMemberSubjectUser, user.ID, role.ID); err != nil {
		t.Fatalf("EnsureProjectMemberRoleBinding(%s, %s, %s) returned error: %v", project.Name, user.Username, role.Name, err)
	}
}

func mustCreateScenarioAssetGroup(t *testing.T, projectID int, name, slug string) *AssetGroup {
	t.Helper()

	group, err := CreateAssetGroup(AssetGroupCreateInput{
		ProjectID: projectID,
		Name:      name,
		Slug:      slug,
	})
	if err != nil {
		t.Fatalf("CreateAssetGroup(%s) returned error: %v", name, err)
	}
	return group
}

func mustAttachScenarioResource(t *testing.T, group *AssetGroup, resource *Resource) {
	t.Helper()

	if _, err := EnsureAssetGroupResource(group.ID, resource.ID); err != nil {
		t.Fatalf("EnsureAssetGroupResource(%s, %s) returned error: %v", group.Name, resource.Name, err)
	}
}

func mustAssignScenarioResource(t *testing.T, project *Project, resource *Resource, subjectType RoleBindingSubject, subjectID int, role *Role) {
	t.Helper()

	resourceID := resource.ID
	if _, _, err := EnsureAssetAssignment(AssetAssignmentInput{
		ProjectID:   project.ID,
		ResourceID:  &resourceID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      role.ID,
	}); err != nil {
		t.Fatalf("EnsureAssetAssignment(%s, %s) returned error: %v", resource.Name, role.Name, err)
	}
}

func mustAssignScenarioAssetGroup(t *testing.T, project *Project, group *AssetGroup, subjectType RoleBindingSubject, subjectID int, role *Role) {
	t.Helper()

	assetGroupID := group.ID
	if _, _, err := EnsureAssetAssignment(AssetAssignmentInput{
		ProjectID:    project.ID,
		AssetGroupID: &assetGroupID,
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		RoleID:       role.ID,
	}); err != nil {
		t.Fatalf("EnsureAssetAssignment(%s, %s) returned error: %v", group.Name, role.Name, err)
	}
}

func scenarioCopyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
