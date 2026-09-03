package db

import "time"

type (
	GroupType             uint8
	RoleBindingSubject    uint8
	RoleBindingScope      uint8
	ProjectType           uint8
	ProjectMemberSubject  uint8
	ProjectRole           uint8
	OwnerSubjectType      uint8
	OwnerType             uint8
	ResourceType          uint8
	ResourceStatus        uint8
	NetworkType           uint8
	PowerState            uint8
	JobType               uint8
	JobStatus             uint8
	JobLogStream          uint8
	SecretType            uint8
	SecretOwnerType       uint8
	QuotaReservationState uint8
	MembershipRole        uint8
	ProxmoxDriftState     string

	User struct {
		ID            int       `gosqlite:"id,primary,increment" json:"id"`
		UUID          string    `gosqlite:"uuid,unique,notnull" json:"uuid"`
		Username      string    `gosqlite:"username,unique,notnull" json:"username"`
		DisplayName   string    `gosqlite:"display_name" json:"display_name"`
		Email         string    `gosqlite:"email" json:"email"`
		AuthSource    string    `gosqlite:"auth_source,notnull" json:"auth_source"`
		ExternalID    string    `gosqlite:"external_id" json:"external_id"`
		IsActive      bool      `gosqlite:"is_active,notnull" json:"is_active"`
		IsSystemAdmin bool      `gosqlite:"is_system_admin,notnull" json:"is_system_admin"`
		CreatedAt     time.Time `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	CloudGroup struct {
		ID             int              `gosqlite:"id,primary,increment" json:"id"`
		UUID           string           `gosqlite:"uuid,unique,notnull" json:"uuid"`
		Name           string           `gosqlite:"name,notnull" json:"name"`
		Slug           string           `gosqlite:"slug,unique,notnull" json:"slug"`
		Description    string           `gosqlite:"description" json:"description"`
		GroupType      GroupType        `gosqlite:"group_type,notnull" json:"group_type"`
		ParentGroupID  *int             `gosqlite:"parent_group_id,fkey:CloudGroup.id" json:"parent_group_id,omitempty"`
		OwnerScopeType RoleBindingScope `gosqlite:"owner_scope_type,notnull" json:"owner_scope_type"`
		OwnerScopeID   *int             `gosqlite:"owner_scope_id" json:"owner_scope_id,omitempty"`
		SyncSource     string           `gosqlite:"sync_source" json:"sync_source"`
		ExternalID     string           `gosqlite:"external_id" json:"external_id"`
		SyncMembership bool             `gosqlite:"sync_membership" json:"sync_membership"`
		CreatedAt      time.Time        `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time        `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt     *time.Time       `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	CloudGroupMembership struct {
		ID             int            `gosqlite:"id,primary,increment" json:"id"`
		UserID         int            `gosqlite:"user_id,fkey:User.id,notnull" json:"user_id"`
		GroupID        int            `gosqlite:"group_id,fkey:CloudGroup.id,notnull" json:"group_id"`
		MembershipRole MembershipRole `gosqlite:"membership_role,notnull" json:"membership_role"`
		CreatedAt      time.Time      `gosqlite:"created_at,notnull" json:"created_at"`
	}

	Organization struct {
		ID          int        `gosqlite:"id,primary,increment" json:"id"`
		UUID        string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
		Name        string     `gosqlite:"name,notnull" json:"name"`
		Slug        string     `gosqlite:"slug,unique,notnull" json:"slug"`
		Description string     `gosqlite:"description" json:"description"`
		ParentOrgID *int       `gosqlite:"parent_org_id,fkey:Organization.id" json:"parent_org_id,omitempty"`
		CreatedAt   time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt  *time.Time `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	OrganizationMembership struct {
		ID             int                  `gosqlite:"id,primary,increment" json:"id"`
		OrganizationID int                  `gosqlite:"organization_id,fkey:Organization.id,notnull" json:"organization_id"`
		SubjectType    ProjectMemberSubject `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID      int                  `gosqlite:"subject_id,notnull" json:"subject_id"`
		MembershipRole MembershipRole       `gosqlite:"membership_role,notnull" json:"membership_role"`
		CreatedAt      time.Time            `gosqlite:"created_at,notnull" json:"created_at"`
	}

	Project struct {
		ID             int         `gosqlite:"id,primary,increment" json:"id"`
		UUID           string      `gosqlite:"uuid,unique,notnull" json:"uuid"`
		OrganizationID int         `gosqlite:"organization_id,fkey:Organization.id,notnull" json:"organization_id"`
		Name           string      `gosqlite:"name,notnull" json:"name"`
		Slug           string      `gosqlite:"slug,unique,notnull" json:"slug"`
		ProjectType    ProjectType `gosqlite:"project_type,notnull" json:"project_type"`
		Description    string      `gosqlite:"description" json:"description"`
		IsActive       bool        `gosqlite:"is_active,notnull" json:"is_active"`
		CreatedAt      time.Time   `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time   `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	ProjectMembership struct {
		ID          int                  `gosqlite:"id,primary,increment" json:"id"`
		ProjectID   int                  `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		SubjectType ProjectMemberSubject `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID   int                  `gosqlite:"subject_id,notnull" json:"subject_id"`
		CreatedAt   time.Time            `gosqlite:"created_at,notnull" json:"created_at"`
	}

	Role struct {
		ID              int              `gosqlite:"id,primary,increment" json:"id"`
		Name            string           `gosqlite:"name,unique,notnull" json:"name"`
		Description     string           `gosqlite:"description" json:"description"`
		IsSystemRole    bool             `gosqlite:"is_system_role,notnull" json:"is_system_role"`
		OwnerScopeType  RoleBindingScope `gosqlite:"owner_scope_type,notnull" json:"owner_scope_type"`
		OwnerScopeID    *int             `gosqlite:"owner_scope_id" json:"owner_scope_id,omitempty"`
		CreatedByUserID *int             `gosqlite:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time        `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt       time.Time        `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	Permission struct {
		ID          int    `gosqlite:"id,primary,increment" json:"id"`
		Name        string `gosqlite:"name,unique,notnull" json:"name"`
		Description string `gosqlite:"description" json:"description"`
	}

	RolePermission struct {
		ID           int `gosqlite:"id,primary,increment" json:"id"`
		RoleID       int `gosqlite:"role_id,fkey:Role.id,notnull" json:"role_id"`
		PermissionID int `gosqlite:"permission_id,fkey:Permission.id,notnull" json:"permission_id"`
	}

	RoleBinding struct {
		ID              int                `gosqlite:"id,primary,increment" json:"id"`
		RoleID          int                `gosqlite:"role_id,fkey:Role.id,notnull" json:"role_id"`
		SubjectType     RoleBindingSubject `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID       int                `gosqlite:"subject_id,notnull" json:"subject_id"`
		ScopeType       RoleBindingScope   `gosqlite:"scope_type,notnull" json:"scope_type"`
		ScopeID         *int               `gosqlite:"scope_id" json:"scope_id,omitempty"`
		SourceType      string             `gosqlite:"source_type" json:"source_type,omitempty"`
		SourceID        *int               `gosqlite:"source_id" json:"source_id,omitempty"`
		CreatedByUserID *int               `gosqlite:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time          `gosqlite:"created_at,notnull" json:"created_at"`
	}

	QuotaPolicy struct {
		ID            int        `gosqlite:"id,primary,increment" json:"id"`
		Name          string     `gosqlite:"name,notnull" json:"name"`
		Description   string     `gosqlite:"description" json:"description"`
		MaxVMs        *int       `gosqlite:"max_vms" json:"max_vms,omitempty"`
		MaxContainers *int       `gosqlite:"max_containers" json:"max_containers,omitempty"`
		MaxVCPU       *int       `gosqlite:"max_vcpu" json:"max_vcpu,omitempty"`
		MaxMemoryMB   *int       `gosqlite:"max_memory_mb" json:"max_memory_mb,omitempty"`
		MaxStorageGB  *int       `gosqlite:"max_storage_gb" json:"max_storage_gb,omitempty"`
		MaxNetworks   *int       `gosqlite:"max_networks" json:"max_networks,omitempty"`
		MaxPublicIPs  *int       `gosqlite:"max_public_ips" json:"max_public_ips,omitempty"`
		CreatedAt     time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt    *time.Time `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	QuotaBinding struct {
		ID            int              `gosqlite:"id,primary,increment" json:"id"`
		QuotaPolicyID int              `gosqlite:"quota_policy_id,fkey:QuotaPolicy.id,notnull" json:"quota_policy_id"`
		SubjectType   RoleBindingScope `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID     int              `gosqlite:"subject_id,notnull" json:"subject_id"`
		CreatedAt     time.Time        `gosqlite:"created_at,notnull" json:"created_at"`
	}

	QuotaReservation struct {
		ID         int                   `gosqlite:"id,primary,increment" json:"id"`
		ProjectID  int                   `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		JobID      *int                  `gosqlite:"job_id,fkey:Job.id" json:"job_id,omitempty"`
		Deployment string                `gosqlite:"deployment" json:"deployment"`
		VMs        int                   `gosqlite:"vms,notnull" json:"vms"`
		Containers int                   `gosqlite:"containers,notnull" json:"containers"`
		VCPU       int                   `gosqlite:"vcpu,notnull" json:"vcpu"`
		MemoryMB   int                   `gosqlite:"memory_mb,notnull" json:"memory_mb"`
		StorageGB  int                   `gosqlite:"storage_gb,notnull" json:"storage_gb"`
		Networks   int                   `gosqlite:"networks,notnull" json:"networks"`
		PublicIPs  int                   `gosqlite:"public_ips,notnull" json:"public_ips"`
		State      QuotaReservationState `gosqlite:"state,notnull" json:"state"`
		ExpiresAt  time.Time             `gosqlite:"expires_at,notnull" json:"expires_at"`
		CreatedAt  time.Time             `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt  time.Time             `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	Resource struct {
		ID              int            `gosqlite:"id,primary,increment" json:"id"`
		UUID            string         `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ProjectID       int            `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		OwnerType       OwnerType      `gosqlite:"owner_type,notnull" json:"owner_type"`
		OwnerID         int            `gosqlite:"owner_id,notnull" json:"owner_id"`
		ResourceType    ResourceType   `gosqlite:"resource_type,notnull" json:"resource_type"`
		Name            string         `gosqlite:"name,notnull" json:"name"`
		Slug            string         `gosqlite:"slug" json:"slug"`
		Status          ResourceStatus `gosqlite:"status,notnull" json:"status"`
		CreatedByUserID *int           `gosqlite:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time      `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt       time.Time      `gosqlite:"updated_at,notnull" json:"updated_at"`
		DeletedAt       *time.Time     `gosqlite:"deleted_at" json:"deleted_at,omitempty"`
	}

	ResourceOwner struct {
		ID          int              `gosqlite:"id,primary,increment" json:"id"`
		ResourceID  int              `gosqlite:"resource_id,fkey:Resource.id,notnull" json:"resource_id"`
		SubjectType OwnerSubjectType `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID   int              `gosqlite:"subject_id,notnull" json:"subject_id"`
		CreatedAt   time.Time        `gosqlite:"created_at,notnull" json:"created_at"`
	}

	AssetGroup struct {
		ID          int        `gosqlite:"id,primary,increment" json:"id"`
		UUID        string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ProjectID   int        `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		Name        string     `gosqlite:"name,notnull" json:"name"`
		Slug        string     `gosqlite:"slug,notnull" json:"slug"`
		Description string     `gosqlite:"description" json:"description"`
		CreatedAt   time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt  *time.Time `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	AssetGroupResource struct {
		ID           int       `gosqlite:"id,primary,increment" json:"id"`
		AssetGroupID int       `gosqlite:"asset_group_id,fkey:AssetGroup.id,notnull" json:"asset_group_id"`
		ResourceID   int       `gosqlite:"resource_id,fkey:Resource.id,notnull" json:"resource_id"`
		CreatedAt    time.Time `gosqlite:"created_at,notnull" json:"created_at"`
	}

	AssetAssignment struct {
		ID              int                `gosqlite:"id,primary,increment" json:"id"`
		ProjectID       int                `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		ResourceID      *int               `gosqlite:"resource_id,fkey:Resource.id" json:"resource_id,omitempty"`
		AssetGroupID    *int               `gosqlite:"asset_group_id,fkey:AssetGroup.id" json:"asset_group_id,omitempty"`
		SubjectType     RoleBindingSubject `gosqlite:"subject_type,notnull" json:"subject_type"`
		SubjectID       int                `gosqlite:"subject_id,notnull" json:"subject_id"`
		RoleID          int                `gosqlite:"role_id,fkey:Role.id,notnull" json:"role_id"`
		CreatedByUserID *int               `gosqlite:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time          `gosqlite:"created_at,notnull" json:"created_at"`
		ArchivedAt      *time.Time         `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	ProxmoxCluster struct {
		ID                 int       `gosqlite:"id,primary,increment" json:"id"`
		UUID               string    `gosqlite:"uuid,unique,notnull" json:"uuid"`
		Name               string    `gosqlite:"name,notnull" json:"name"`
		APIURL             string    `gosqlite:"api_url,notnull" json:"api_url"`
		VerifyTLS          bool      `gosqlite:"verify_tls,notnull" json:"verify_tls"`
		CredentialSecretID *int      `gosqlite:"credential_secret_id" json:"credential_secret_id,omitempty"`
		IsActive           bool      `gosqlite:"is_active,notnull" json:"is_active"`
		CreatedAt          time.Time `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt          time.Time `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	ProxmoxNode struct {
		ID            int       `gosqlite:"id,primary,increment" json:"id"`
		ClusterID     int       `gosqlite:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		Name          string    `gosqlite:"name,notnull" json:"name"`
		Status        string    `gosqlite:"status" json:"status"`
		CPUTotal      int       `gosqlite:"cpu_total" json:"cpu_total"`
		MemoryTotalMB int       `gosqlite:"memory_total_mb" json:"memory_total_mb"`
		CreatedAt     time.Time `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	ProxmoxInventoryGuest struct {
		ID              int               `gosqlite:"id,primary,increment" json:"id"`
		Identity        string            `gosqlite:"identity,unique,notnull" json:"identity"`
		ClusterIdentity string            `gosqlite:"cluster_identity,notnull" json:"cluster_identity"`
		ProxmoxVMID     int               `gosqlite:"proxmox_vmid,notnull" json:"vmid"`
		ResourceID      *int              `gosqlite:"resource_id,fkey:Resource.id" json:"resource_id,omitempty"`
		Node            string            `gosqlite:"node,notnull" json:"node"`
		Name            string            `gosqlite:"name,notnull" json:"name"`
		Kind            string            `gosqlite:"kind,notnull" json:"kind"`
		IsTemplate      bool              `gosqlite:"is_template,notnull" json:"is_template"`
		Status          string            `gosqlite:"status" json:"status"`
		Tags            string            `gosqlite:"tags" json:"tags"`
		Fingerprint     string            `gosqlite:"fingerprint,notnull" json:"-"`
		DriftState      ProxmoxDriftState `gosqlite:"drift_state,notnull" json:"drift_state"`
		LastError       string            `gosqlite:"last_error" json:"last_error,omitempty"`
		FirstSeenAt     time.Time         `gosqlite:"first_seen_at,notnull" json:"first_seen_at"`
		LastSeenAt      time.Time         `gosqlite:"last_seen_at,notnull" json:"last_seen_at"`
		MissingSince    *time.Time        `gosqlite:"missing_since" json:"missing_since,omitempty"`
		UpdatedAt       time.Time         `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	VirtualMachine struct {
		ID          int        `gosqlite:"id,primary,increment" json:"id"`
		ResourceID  int        `gosqlite:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID   int        `gosqlite:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		NodeID      *int       `gosqlite:"node_id,fkey:ProxmoxNode.id" json:"node_id,omitempty"`
		ProxmoxVMID int        `gosqlite:"proxmox_vmid,notnull" json:"proxmox_vmid"`
		Name        string     `gosqlite:"name,notnull" json:"name"`
		CPUCores    int        `gosqlite:"cpu_cores,notnull" json:"cpu_cores"`
		MemoryMB    int        `gosqlite:"memory_mb,notnull" json:"memory_mb"`
		DiskGB      *int       `gosqlite:"disk_gb" json:"disk_gb,omitempty"`
		OSType      string     `gosqlite:"os_type" json:"os_type"`
		TemplateID  *int       `gosqlite:"template_id" json:"template_id,omitempty"`
		PowerState  PowerState `gosqlite:"power_state,notnull" json:"power_state"`
		CreatedAt   time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	Container struct {
		ID          int        `gosqlite:"id,primary,increment" json:"id"`
		ResourceID  int        `gosqlite:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID   int        `gosqlite:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		NodeID      *int       `gosqlite:"node_id,fkey:ProxmoxNode.id" json:"node_id,omitempty"`
		ProxmoxVMID int        `gosqlite:"proxmox_vmid,notnull" json:"proxmox_vmid"`
		Name        string     `gosqlite:"name,notnull" json:"name"`
		CPUCores    int        `gosqlite:"cpu_cores,notnull" json:"cpu_cores"`
		MemoryMB    int        `gosqlite:"memory_mb,notnull" json:"memory_mb"`
		DiskGB      *int       `gosqlite:"disk_gb" json:"disk_gb,omitempty"`
		TemplateID  *int       `gosqlite:"template_id" json:"template_id,omitempty"`
		PowerState  PowerState `gosqlite:"power_state,notnull" json:"power_state"`
		CreatedAt   time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	VirtualNetwork struct {
		ID                 int         `gosqlite:"id,primary,increment" json:"id"`
		ResourceID         int         `gosqlite:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID          int         `gosqlite:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		Name               string      `gosqlite:"name,notnull" json:"name"`
		NetworkType        NetworkType `gosqlite:"network_type,notnull" json:"network_type"`
		CIDRIPv4           string      `gosqlite:"cidr_ipv4" json:"cidr_ipv4"`
		CIDRIPv6           string      `gosqlite:"cidr_ipv6" json:"cidr_ipv6"`
		VLANID             *int        `gosqlite:"vlan_id" json:"vlan_id,omitempty"`
		VXLANID            *int        `gosqlite:"vxlan_id" json:"vxlan_id,omitempty"`
		GatewayIPv4        string      `gosqlite:"gateway_ipv4" json:"gateway_ipv4"`
		GatewayIPv6        string      `gosqlite:"gateway_ipv6" json:"gateway_ipv6"`
		IsInternetRoutable bool        `gosqlite:"is_internet_routable,notnull" json:"is_internet_routable"`
		CreatedAt          time.Time   `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt          time.Time   `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	Job struct {
		ID                int        `gosqlite:"id,primary,increment" json:"id"`
		UUID              string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
		JobType           JobType    `gosqlite:"job_type,notnull" json:"job_type"`
		Status            JobStatus  `gosqlite:"status,notnull" json:"status"`
		RequestedByUserID *int       `gosqlite:"requested_by_user_id" json:"requested_by_user_id,omitempty"`
		ProjectID         *int       `gosqlite:"project_id,fkey:Project.id" json:"project_id,omitempty"`
		ResourceID        *int       `gosqlite:"resource_id,fkey:Resource.id" json:"resource_id,omitempty"`
		QueueID           string     `gosqlite:"queue_id" json:"queue_id"`
		Operation         string     `gosqlite:"operation" json:"operation"`
		InputJSON         string     `gosqlite:"input_json" json:"-"`
		IdempotencyKey    string     `gosqlite:"idempotency_key" json:"-"`
		OperationKey      string     `gosqlite:"operation_key" json:"operation_key"`
		Node              string     `gosqlite:"node" json:"node,omitempty"`
		Progress          int        `gosqlite:"progress" json:"progress"`
		AttemptCount      int        `gosqlite:"attempt_count" json:"attempt_count"`
		MaxAttempts       int        `gosqlite:"max_attempts" json:"max_attempts"`
		ErrorCode         string     `gosqlite:"error_code" json:"error_code,omitempty"`
		ErrorSummary      string     `gosqlite:"error_summary" json:"error_summary,omitempty"`
		RetryClass        string     `gosqlite:"retry_class" json:"retry_class,omitempty"`
		CancelRequestedAt *time.Time `gosqlite:"cancel_requested_at" json:"cancel_requested_at,omitempty"`
		HeartbeatAt       *time.Time `gosqlite:"heartbeat_at" json:"heartbeat_at,omitempty"`
		LeaseExpiresAt    *time.Time `gosqlite:"lease_expires_at" json:"lease_expires_at,omitempty"`
		StartedAt         *time.Time `gosqlite:"started_at" json:"started_at,omitempty"`
		FinishedAt        *time.Time `gosqlite:"finished_at" json:"finished_at,omitempty"`
		CreatedAt         time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt         time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	JobLog struct {
		ID        int          `gosqlite:"id,primary,increment" json:"id"`
		JobID     int          `gosqlite:"job_id,fkey:Job.id,notnull" json:"job_id"`
		Stream    JobLogStream `gosqlite:"stream,notnull" json:"stream"`
		Message   string       `gosqlite:"message,notnull" json:"message"`
		CreatedAt time.Time    `gosqlite:"created_at,notnull" json:"created_at"`
	}

	AuditEvent struct {
		ID           int       `gosqlite:"id,primary,increment" json:"id"`
		UUID         string    `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ActorUserID  *int      `gosqlite:"actor_user_id" json:"actor_user_id,omitempty"`
		Action       string    `gosqlite:"action,notnull" json:"action"`
		TargetType   string    `gosqlite:"target_type" json:"target_type"`
		TargetID     *int      `gosqlite:"target_id" json:"target_id,omitempty"`
		ProjectID    *int      `gosqlite:"project_id,fkey:Project.id" json:"project_id,omitempty"`
		SourceIP     string    `gosqlite:"source_ip" json:"source_ip"`
		UserAgent    string    `gosqlite:"user_agent" json:"user_agent"`
		MetadataJSON string    `gosqlite:"metadata_json" json:"metadata_json"`
		CreatedAt    time.Time `gosqlite:"created_at,notnull" json:"created_at"`
	}

	Secret struct {
		ID             int             `gosqlite:"id,primary,increment" json:"id"`
		UUID           string          `gosqlite:"uuid,unique,notnull" json:"uuid"`
		Name           string          `gosqlite:"name,notnull" json:"name"`
		SecretType     SecretType      `gosqlite:"secret_type,notnull" json:"secret_type"`
		EncryptedValue []byte          `gosqlite:"encrypted_value,notnull" json:"-"`
		OwnerType      SecretOwnerType `gosqlite:"owner_type,notnull" json:"owner_type"`
		OwnerID        *int            `gosqlite:"owner_id" json:"owner_id,omitempty"`
		CreatedAt      time.Time       `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time       `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt     *time.Time      `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}
)
