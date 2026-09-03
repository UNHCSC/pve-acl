package app

import (
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type (
	quotaPolicyRequest struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		MaxVMs        *int   `json:"maxVMs"`
		MaxContainers *int   `json:"maxContainers"`
		MaxVCPU       *int   `json:"maxVCPU"`
		MaxMemoryMB   *int   `json:"maxMemoryMB"`
		MaxStorageGB  *int   `json:"maxStorageGB"`
		MaxNetworks   *int   `json:"maxNetworks"`
		MaxPublicIPs  *int   `json:"maxPublicIPs"`
	}

	quotaBindingRequest struct {
		ScopeType string `json:"scopeType"`
		ScopeID   int    `json:"scopeID"`
	}

	secretRequest struct {
		Name       string `json:"name"`
		SecretType string `json:"secretType"`
		Value      string `json:"value"`
	}
)

// getQuotaPolicies lists active quota policies for quota administrators.
func getQuotaPolicies(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaRead); errResult != nil {
		return
	}
	var (
		policies []*db.QuotaPolicy
		err      error
	)

	policies, err = db.QuotaPolicies.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load quota policies"})
	}
	var active []*db.QuotaPolicy = make([]*db.QuotaPolicy, 0, len(policies))
	for _, policy := range policies {
		if policy.ArchivedAt == nil {
			active = append(active, policy)
		}
	}
	return c.JSON(active)
}

// postQuotaPolicy creates a quota policy.
func postQuotaPolicy(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaUpdate); errResult != nil {
		return
	}
	var req quotaPolicyRequest
	if errResult = c.BodyParser(&req); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota policy"})
	}
	var policy *db.QuotaPolicy
	policy, errResult = db.CreateQuotaPolicy(quotaPolicyInput(req))
	if errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "quota.policy.create", "quota_policy", &policy.ID, nil, map[string]any{"name": policy.Name})
	return c.Status(fiber.StatusCreated).JSON(policy)
}

// patchQuotaPolicy updates a quota policy.
func patchQuotaPolicy(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaUpdate); errResult != nil {
		return
	}
	var (
		policy *db.QuotaPolicy
		id     int
	)
	if id, errResult = strconv.Atoi(c.Params("policyID")); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota policy id"})
	}
	policy, errResult = db.QuotaPolicies.Select(id)
	if errResult != nil || policy == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "quota policy not found"})
	}
	var req quotaPolicyRequest
	if errResult = c.BodyParser(&req); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota policy"})
	}
	if errResult = db.UpdateQuotaPolicy(policy, quotaPolicyInput(req)); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "quota.policy.update", "quota_policy", &policy.ID, nil, map[string]any{"name": policy.Name})
	return c.JSON(policy)
}

// deleteQuotaPolicy archives a quota policy.
func deleteQuotaPolicy(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaUpdate); errResult != nil {
		return
	}
	var id int
	if id, errResult = strconv.Atoi(c.Params("policyID")); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota policy id"})
	}
	var policy *db.QuotaPolicy
	policy, errResult = db.QuotaPolicies.Select(id)
	if errResult != nil || policy == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "quota policy not found"})
	}
	if errResult = db.ArchiveQuotaPolicy(policy); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "quota.policy.archive", "quota_policy", &policy.ID, nil, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// postQuotaPolicyBinding binds a quota policy to a supported scope.
func postQuotaPolicyBinding(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaUpdate); errResult != nil {
		return
	}
	var id int
	if id, errResult = strconv.Atoi(c.Params("policyID")); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota policy id"})
	}
	var req quotaBindingRequest
	if errResult = c.BodyParser(&req); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota binding"})
	}
	var scope db.RoleBindingScope
	switch strings.ToLower(strings.TrimSpace(req.ScopeType)) {
	case "organization", "org":
		scope = db.RoleBindingScopeOrg
	case "project":
		scope = db.RoleBindingScopeProject
	case "group":
		scope = db.RoleBindingScopeGroup
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported quota scope"})
	}
	var binding *db.QuotaBinding
	binding, errResult = db.BindQuotaPolicy(id, scope, req.ScopeID)
	if errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "quota.binding.update", "quota_binding", &binding.ID, nil, map[string]any{"scopeType": req.ScopeType, "scopeID": req.ScopeID})
	return c.Status(fiber.StatusCreated).JSON(binding)
}

// deleteQuotaPolicyBinding removes an effective quota binding.
func deleteQuotaPolicyBinding(c *fiber.Ctx) (errResult error) {
	if errResult = requireGlobalPermission(c, db.PermissionQuotaUpdate); errResult != nil {
		return
	}
	var id int

	if id, errResult = strconv.Atoi(c.Params("bindingID")); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid quota binding id"})
	}
	var binding *db.QuotaBinding

	binding, errResult = db.QuotaBindings.Select(id)
	if errResult != nil || binding == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "quota binding not found"})
	}
	if errResult = db.QuotaBindings.Delete(id); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove quota binding"})
	}
	_ = auditRequest(c, "quota.binding.delete", "quota_binding", &binding.ID, nil, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// getProjectQuota returns effective quota and current intended usage.
func getProjectQuota(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = projectFromIDParam(c)
	if errResult != nil {
		return projectParamError(c, errResult)
	}
	var allowed bool
	allowed, errResult = currentUserCan(c, db.PermissionQuotaRead, db.RoleBindingScopeProject, &project.ID)
	if errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		allowed, errResult = currentUserCanManageProject(c, project)
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var policy *db.QuotaPolicy
	var usage db.QuotaDimensions
	policy, errResult = db.EffectiveProjectQuota(project.ID)
	if errResult == nil {
		usage, errResult = db.ProjectQuotaUsage(project.ID)
	}
	if errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to resolve quota"})
	}
	return c.JSON(fiber.Map{"policy": policy, "usage": usage})
}

// getProjectAudit lists scoped audit history.
func getProjectAudit(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = projectFromIDParam(c)
	if errResult != nil {
		return projectParamError(c, errResult)
	}
	var allowed bool
	allowed, errResult = currentUserCan(c, db.PermissionAuditRead, db.RoleBindingScopeProject, &project.ID)
	if !allowed {
		allowed, errResult = currentUserCanManageProject(c, project)
	}
	if errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var events []*db.AuditEvent
	events, errResult = db.AuditEventsForProject(&project.ID)
	if errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load audit history"})
	}
	return c.JSON(events)
}

// getProjectSecrets lists project secret metadata.
func getProjectSecrets(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = managedProjectFromRequest(c)
	if errResult != nil {
		return
	}
	var secrets []*db.Secret
	secrets, errResult = db.ActiveSecrets()
	if errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load secrets"})
	}
	var items []fiber.Map = []fiber.Map{}
	for _, secret := range secrets {
		if secret.OwnerType == db.SecretOwnerTypeProject && secret.OwnerID != nil && *secret.OwnerID == project.ID {
			items = append(items, secretResponse(secret))
		}
	}
	return c.JSON(items)
}

// postProjectSecret creates an encrypted project secret.
func postProjectSecret(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = managedProjectFromRequest(c)
	if errResult != nil {
		return
	}
	var req secretRequest
	if errResult = c.BodyParser(&req); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid secret request"})
	}
	var secretType db.SecretType
	if secretType, errResult = parseSecretType(req.SecretType); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	var secret *db.Secret
	secret, errResult = db.CreateSecret(db.SecretCreateInput{Name: req.Name, SecretType: secretType, Value: req.Value, OwnerType: db.SecretOwnerTypeProject, OwnerID: &project.ID})
	if errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "secret.create", "secret", &secret.ID, &project.ID, map[string]any{"name": secret.Name})
	return c.Status(fiber.StatusCreated).JSON(secretResponse(secret))
}

// patchProjectSecret rotates an encrypted project secret.
func patchProjectSecret(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = managedProjectFromRequest(c)
	if errResult != nil {
		return
	}
	var secret *db.Secret
	secret, errResult = projectSecretFromParam(c, project.ID)
	if errResult != nil {
		return
	}
	var req secretRequest
	if errResult = c.BodyParser(&req); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid secret request"})
	}
	if errResult = db.RotateSecret(secret, req.Value); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "secret.rotate", "secret", &secret.ID, &project.ID, nil)
	return c.JSON(secretResponse(secret))
}

// deleteProjectSecret archives an encrypted project secret.
func deleteProjectSecret(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	project, errResult = managedProjectFromRequest(c)
	if errResult != nil {
		return
	}
	var secret *db.Secret
	secret, errResult = projectSecretFromParam(c, project.ID)
	if errResult != nil {
		return
	}
	if errResult = db.ArchiveSecret(secret); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "secret.archive", "secret", &secret.ID, &project.ID, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

func quotaPolicyInput(req quotaPolicyRequest) db.QuotaPolicyInput {
	return db.QuotaPolicyInput{Name: req.Name, Description: req.Description, MaxVMs: req.MaxVMs, MaxContainers: req.MaxContainers, MaxVCPU: req.MaxVCPU, MaxMemoryMB: req.MaxMemoryMB, MaxStorageGB: req.MaxStorageGB, MaxNetworks: req.MaxNetworks, MaxPublicIPs: req.MaxPublicIPs}
}
func requireGlobalPermission(c *fiber.Ctx, permission db.PermissionKey) (errResult error) {
	var (
		allowed bool
		err     error
	)

	allowed, err = currentUserCan(c, permission, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(403).JSON(fiber.Map{"error": "permission denied"})
	}
	return nil
}
func managedProjectFromRequest(c *fiber.Ctx) (projectResult *db.Project, errResult error) {
	var (
		project *db.Project
		allowed bool
		err     error
	)

	project, err = projectFromIDParam(c)
	if err != nil {
		return nil, projectParamError(c, err)
	}
	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return nil, c.Status(500).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return nil, c.Status(403).JSON(fiber.Map{"error": "permission denied"})
	}
	return project, nil
}
func secretResponse(secret *db.Secret) fiber.Map {
	return fiber.Map{"id": secret.ID, "uuid": secret.UUID, "name": secret.Name, "secret_type": secret.SecretType, "owner_type": secret.OwnerType, "owner_id": secret.OwnerID, "created_at": secret.CreatedAt, "updated_at": secret.UpdatedAt}
}
func parseSecretType(value string) (db.SecretType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "proxmox-token":
		return db.SecretTypeProxmoxToken, nil
	case "ssh-key":
		return db.SecretTypeSSHKey, nil
	case "password":
		return db.SecretTypePassword, nil
	case "api-token":
		return db.SecretTypeAPIToken, nil
	case "terraform-var":
		return db.SecretTypeTerraformVar, nil
	case "ansible-var":
		return db.SecretTypeAnsibleVar, nil
	}
	return 0, fiber.NewError(400, "unsupported secret type")
}
func projectSecretFromParam(c *fiber.Ctx, projectID int) (secretResult *db.Secret, errResult error) {
	var (
		id     int
		secret *db.Secret
		err    error
	)

	id, err = strconv.Atoi(c.Params("secretID"))
	if err != nil {
		return nil, c.Status(400).JSON(fiber.Map{"error": "invalid secret id"})
	}
	secret, err = db.Secrets.Select(id)
	if err != nil {
		return nil, c.Status(500).JSON(fiber.Map{"error": "failed to load secret"})
	}
	if secret == nil || secret.ArchivedAt != nil || secret.OwnerType != db.SecretOwnerTypeProject || secret.OwnerID == nil || *secret.OwnerID != projectID {
		return nil, c.Status(404).JSON(fiber.Map{"error": "secret not found"})
	}
	return secret, nil
}
func auditRequest(c *fiber.Ctx, action, targetType string, targetID, projectID *int, metadata map[string]any) (errResult error) {
	var actorID *int
	var user *db.User

	if user = currentDBUser(c); user != nil {
		actorID = &user.ID
	}
	_, errResult = db.WriteAudit(db.AuditInput{ActorUserID: actorID, Action: action, TargetType: targetType, TargetID: targetID, ProjectID: projectID, SourceIP: c.IP(), UserAgent: c.Get("User-Agent"), Metadata: metadata})
	return
}
