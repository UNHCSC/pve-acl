package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestBlueprintVersionsAreImmutableAndPreviewExpandsGroups(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project = createResourceAPIProject(t, "Blueprint Course")
	var manager *db.User = ensureResourceAPIUser(t, "blueprint-manager")
	grantProjectRole(t, project.ID, manager.ID, db.ProjectRoleManager)
	var now time.Time = time.Now().UTC()
	var err error
	var groups []*db.CloudGroup
	for index := 1; index <= 3; index++ {
		var group *db.CloudGroup = &db.CloudGroup{UUID: fmt.Sprintf("blueprint-group-%d", index), Name: fmt.Sprintf("Team %d", index), Slug: fmt.Sprintf("team-%d", index), GroupType: db.GroupTypeStudentGroup, OwnerScopeType: db.RoleBindingScopeProject, OwnerScopeID: &project.ID, SyncSource: db.CloudGroupSyncSourceLocal, CreatedAt: now, UpdatedAt: now}
		if err = db.CloudGroups.Insert(group); err != nil {
			t.Fatalf("insert group: %v", err)
		}
		groups = append(groups, group)
	}
	var fiberApp *fiber.App = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:id/blueprints", getProjectBlueprints)
	fiberApp.Post("/api/v1/projects/:id/blueprints", postProjectBlueprint)
	fiberApp.Post("/api/v1/blueprints/:id/versions", postBlueprintVersion)
	fiberApp.Post("/api/v1/projects/:id/deployment-previews", postProjectDeploymentPreview)
	fiberApp.Get("/api/v1/projects/:id/allocation-pools", getProjectAllocationPools)
	fiberApp.Post("/api/v1/projects/:id/allocation-pools", postProjectAllocationPool)
	var token string = authenticateTestUser(t, manager.Username, false)
	var response *http.Response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/blueprints", `{"name":"Generic lab","slug":"generic-lab"}`)
	if response.StatusCode != fiber.StatusCreated {
		t.Fatalf("create blueprint status=%d", response.StatusCode)
	}
	var blueprint db.Blueprint
	if err = json.NewDecoder(response.Body).Decode(&blueprint); err != nil {
		t.Fatal(err)
	}
	var resources []string
	for index := 1; index <= 8; index++ {
		resources = append(resources, fmt.Sprintf(`{"key":"vm-%d","kind":"vm","template":"template-v1","vcpu":1,"memory_mb":1024,"disk_gb":10,"networks":["lan"]}`, index))
	}
	var document string = fmt.Sprintf(`{"document":{"format_version":1,"opentofu_module":"ssh://example/module?ref=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","ansible_project":"ssh://example/ansible?ref=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","name_pattern":"{{deployment}}-{{resource}}","resources":[%s],"networks":[{"key":"lan","kind":"isolated","ipv4_cidr":"192.168.1.0/24"}]}}`, strings.Join(resources, ","))
	response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/blueprints/"+strconv.Itoa(blueprint.ID)+"/versions", document)
	if response.StatusCode != fiber.StatusCreated {
		t.Fatalf("publish status=%d", response.StatusCode)
	}
	var version map[string]any
	if err = json.NewDecoder(response.Body).Decode(&version); err != nil {
		t.Fatal(err)
	}
	response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/blueprints/"+strconv.Itoa(blueprint.ID)+"/versions", document)
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("duplicate immutable document status=%d", response.StatusCode)
	}
	var groupIDs []string
	for _, group := range groups {
		groupIDs = append(groupIDs, strconv.Itoa(group.ID))
	}
	var previewBody string = fmt.Sprintf(`{"blueprintVersionID":%d,"groupIDs":[%s],"namePrefix":"it666-fa26"}`, int(version["id"].(float64)), strings.Join(groupIDs, ","))
	response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/deployment-previews", previewBody)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("preview status=%d", response.StatusCode)
	}
	var preview map[string]any
	if err = json.NewDecoder(response.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	var deployments []any = preview["deployments"].([]any)
	var totals map[string]any = preview["totals"].(map[string]any)
	if len(deployments) != 3 || totals["vms"] != float64(24) || preview["mutates"] != false {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	var stored *db.BlueprintVersion
	if stored, _ = db.BlueprintVersions.Select(int(version["id"].(float64))); stored == nil || stored.DocumentJSON == "" {
		t.Fatal("published document was not persisted")
	}
	response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/allocation-pools", `{"name":"Tiny VMIDs","kind":"vmid","start":900,"end":901}`)
	if response.StatusCode != fiber.StatusCreated {
		t.Fatalf("create allocation pool status=%d", response.StatusCode)
	}
	var pool db.AllocationPool
	if err = json.NewDecoder(response.Body).Decode(&pool); err != nil {
		t.Fatal(err)
	}
	previewBody = fmt.Sprintf(`{"blueprintVersionID":%d,"groupIDs":[%s],"namePrefix":"it666-fa26","allocationPoolIDs":{"vmid":%d}}`, int(version["id"].(float64)), strings.Join(groupIDs, ","), pool.ID)
	response = resourceAPIRequest(t, fiberApp, token, http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/deployment-previews", previewBody)
	if response.StatusCode != fiber.StatusConflict {
		t.Fatalf("expected exhausted VMID preflight conflict, got %d", response.StatusCode)
	}
	var vmidPool *db.AllocationPool
	var vlanPool *db.AllocationPool
	if vmidPool, err = db.CreateAllocationPool(project.ID, "Deployment VMIDs", "vmid", 1000, 1023, ""); err != nil {
		t.Fatal(err)
	}
	if vlanPool, err = db.CreateAllocationPool(project.ID, "Deployment VLANs", "vlan", 200, 202, ""); err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	var results chan error = make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func(value int) {
			var planErr error
			_, planErr = db.CreateDeploymentPlan(db.DeploymentPlanInput{ProjectID: project.ID, BlueprintVersionID: int(version["id"].(float64)), GroupIDs: ids, NamePrefix: fmt.Sprintf("race-%d", value), AllocationPoolIDs: map[string]int{"vmid": vmidPool.ID, "vlan": vlanPool.ID}})
			results <- planErr
		}(index)
	}
	var successes int
	for index := 0; index < 2; index++ {
		if err = <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one concurrent plan to reserve capacity, got %d", successes)
	}
	var allocations []*db.Allocation
	if allocations, err = db.Allocations.SelectAll(); err != nil || len(allocations) != 27 {
		t.Fatalf("expected 27 durable unique allocations, count=%d err=%v", len(allocations), err)
	}
	var values map[string]bool = make(map[string]bool, len(allocations))
	for _, allocation := range allocations {
		if values[allocation.AllocationKey] {
			t.Fatalf("duplicate allocation key %s", allocation.AllocationKey)
		}
		values[allocation.AllocationKey] = true
	}
	var persistedDeployments []*db.Deployment
	if persistedDeployments, err = db.DeploymentsForProject(project.ID); err != nil || len(persistedDeployments) != 3 {
		t.Fatalf("expected three persisted deployments, count=%d err=%v", len(persistedDeployments), err)
	}
	var rollbackVMIDs *db.AllocationPool
	var rollbackVLANs *db.AllocationPool
	if rollbackVMIDs, err = db.CreateAllocationPool(project.ID, "Rollback VMIDs", "vmid", 3000, 3023, ""); err != nil {
		t.Fatal(err)
	}
	if rollbackVLANs, err = db.CreateAllocationPool(project.ID, "Rollback VLANs", "vlan", 300, 302, ""); err != nil {
		t.Fatal(err)
	}
	var existingPrefix string = strings.TrimSuffix(persistedDeployments[0].Name, "-g01")
	if _, err = db.CreateDeploymentPlan(db.DeploymentPlanInput{ProjectID: project.ID, BlueprintVersionID: int(version["id"].(float64)), GroupIDs: ids, NamePrefix: existingPrefix, AllocationPoolIDs: map[string]int{"vmid": rollbackVMIDs.ID, "vlan": rollbackVLANs.ID}}); err == nil {
		t.Fatal("expected duplicate deployment names to roll back")
	}
	if allocations, err = db.Allocations.SelectAll(); err != nil || len(allocations) != 27 {
		t.Fatalf("failed plan leaked allocations, count=%d err=%v", len(allocations), err)
	}
	var reservations []*db.QuotaReservation
	if reservations, err = db.QuotaReservations.SelectAll(); err != nil || len(reservations) != 2 || reservations[1].State != db.QuotaReservationReleased {
		t.Fatalf("failed plan did not release quota: %#v err=%v", reservations, err)
	}
}

func TestBlueprintDocumentRejectsInvalidReferences(t *testing.T) {
	var document db.BlueprintDocument = db.BlueprintDocument{FormatVersion: 1, OpenTofuModule: "module", AnsibleProject: "playbook", NamePattern: "{{deployment}}-{{resource}}", Resources: []db.BlueprintResourceSpec{{Key: "vm", Kind: "vm", Template: "template", VCPU: 1, MemoryMB: 512, DiskGB: 8, Networks: []string{"missing"}}}}
	var err error
	if err = db.ValidateBlueprintDocument(document); err == nil {
		t.Fatal("expected unknown network validation failure")
	}
}
