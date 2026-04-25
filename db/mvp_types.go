package db

import "time"

type (
	GroupType            uint8
	RoleBindingSubject   uint8
	RoleBindingScope     uint8
	ProjectType          uint8
	ProjectMemberSubject uint8
	ProjectRole          uint8
	OwnerType            uint8
	ResourceType         uint8
	ResourceStatus       uint8
	NetworkType          uint8
	PowerState           uint8
	JobType              uint8
	JobStatus            uint8
	JobLogStream         uint8
	SecretType           uint8
	SecretOwnerType      uint8
	MembershipRole       uint8

	User struct {
		ID            int       `gomysql:"id,primary,increment" json:"id"`
		UUID          string    `gomysql:"uuid,unique,notnull" json:"uuid"`
		Username      string    `gomysql:"username,unique,notnull" json:"username"`
		DisplayName   string    `gomysql:"display_name" json:"display_name"`
		Email         string    `gomysql:"email" json:"email"`
		AuthSource    string    `gomysql:"auth_source,notnull" json:"auth_source"`
		ExternalID    string    `gomysql:"external_id" json:"external_id"`
		IsActive      bool      `gomysql:"is_active,notnull" json:"is_active"`
		IsSystemAdmin bool      `gomysql:"is_system_admin,notnull" json:"is_system_admin"`
		CreatedAt     time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	CloudGroup struct {
		ID             int       `gomysql:"id,primary,increment" json:"id"`
		UUID           string    `gomysql:"uuid,unique,notnull" json:"uuid"`
		Name           string    `gomysql:"name,notnull" json:"name"`
		Slug           string    `gomysql:"slug,unique,notnull" json:"slug"`
		Description    string    `gomysql:"description" json:"description"`
		GroupType      GroupType `gomysql:"group_type,notnull" json:"group_type"`
		ParentGroupID  *int      `gomysql:"parent_group_id,fkey:CloudGroup.id" json:"parent_group_id,omitempty"`
		SyncSource     string    `gomysql:"sync_source" json:"sync_source"`
		ExternalID     string    `gomysql:"external_id" json:"external_id"`
		SyncMembership bool      `gomysql:"sync_membership" json:"sync_membership"`
		CreatedAt      time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	CloudGroupMembership struct {
		ID             int            `gomysql:"id,primary,increment" json:"id"`
		UserID         int            `gomysql:"user_id,fkey:User.id,notnull" json:"user_id"`
		GroupID        int            `gomysql:"group_id,fkey:CloudGroup.id,notnull" json:"group_id"`
		MembershipRole MembershipRole `gomysql:"membership_role,notnull" json:"membership_role"`
		CreatedAt      time.Time      `gomysql:"created_at,notnull" json:"created_at"`
	}

	Organization struct {
		ID          int       `gomysql:"id,primary,increment" json:"id"`
		UUID        string    `gomysql:"uuid,unique,notnull" json:"uuid"`
		Name        string    `gomysql:"name,notnull" json:"name"`
		Slug        string    `gomysql:"slug,unique,notnull" json:"slug"`
		Description string    `gomysql:"description" json:"description"`
		ParentOrgID *int      `gomysql:"parent_org_id,fkey:Organization.id" json:"parent_org_id,omitempty"`
		CreatedAt   time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	Course struct {
		ID             int        `gomysql:"id,primary,increment" json:"id"`
		UUID           string     `gomysql:"uuid,unique,notnull" json:"uuid"`
		OrganizationID int        `gomysql:"organization_id,fkey:Organization.id,notnull" json:"organization_id"`
		Code           string     `gomysql:"code,notnull" json:"code"`
		Name           string     `gomysql:"name,notnull" json:"name"`
		Semester       string     `gomysql:"semester,notnull" json:"semester"`
		Slug           string     `gomysql:"slug,unique,notnull" json:"slug"`
		StartDate      *time.Time `gomysql:"start_date" json:"start_date,omitempty"`
		EndDate        *time.Time `gomysql:"end_date" json:"end_date,omitempty"`
		CreatedAt      time.Time  `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time  `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	Project struct {
		ID             int         `gomysql:"id,primary,increment" json:"id"`
		UUID           string      `gomysql:"uuid,unique,notnull" json:"uuid"`
		OrganizationID int         `gomysql:"organization_id,fkey:Organization.id,notnull" json:"organization_id"`
		CourseID       *int        `gomysql:"course_id,fkey:Course.id" json:"course_id,omitempty"`
		Name           string      `gomysql:"name,notnull" json:"name"`
		Slug           string      `gomysql:"slug,unique,notnull" json:"slug"`
		ProjectType    ProjectType `gomysql:"project_type,notnull" json:"project_type"`
		Description    string      `gomysql:"description" json:"description"`
		IsActive       bool        `gomysql:"is_active,notnull" json:"is_active"`
		CreatedAt      time.Time   `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time   `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	ProjectMembership struct {
		ID          int                  `gomysql:"id,primary,increment" json:"id"`
		ProjectID   int                  `gomysql:"project_id,fkey:Project.id,notnull" json:"project_id"`
		SubjectType ProjectMemberSubject `gomysql:"subject_type,notnull" json:"subject_type"`
		SubjectID   int                  `gomysql:"subject_id,notnull" json:"subject_id"`
		ProjectRole ProjectRole          `gomysql:"project_role,notnull" json:"project_role"`
		CreatedAt   time.Time            `gomysql:"created_at,notnull" json:"created_at"`
	}

	Role struct {
		ID           int       `gomysql:"id,primary,increment" json:"id"`
		Name         string    `gomysql:"name,unique,notnull" json:"name"`
		Description  string    `gomysql:"description" json:"description"`
		IsSystemRole bool      `gomysql:"is_system_role,notnull" json:"is_system_role"`
		CreatedAt    time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt    time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	Permission struct {
		ID          int    `gomysql:"id,primary,increment" json:"id"`
		Name        string `gomysql:"name,unique,notnull" json:"name"`
		Description string `gomysql:"description" json:"description"`
	}

	RolePermission struct {
		ID           int `gomysql:"id,primary,increment" json:"id"`
		RoleID       int `gomysql:"role_id,fkey:Role.id,notnull" json:"role_id"`
		PermissionID int `gomysql:"permission_id,fkey:Permission.id,notnull" json:"permission_id"`
	}

	RoleBinding struct {
		ID              int                `gomysql:"id,primary,increment" json:"id"`
		RoleID          int                `gomysql:"role_id,fkey:Role.id,notnull" json:"role_id"`
		SubjectType     RoleBindingSubject `gomysql:"subject_type,notnull" json:"subject_type"`
		SubjectID       int                `gomysql:"subject_id,notnull" json:"subject_id"`
		ScopeType       RoleBindingScope   `gomysql:"scope_type,notnull" json:"scope_type"`
		ScopeID         *int               `gomysql:"scope_id" json:"scope_id,omitempty"`
		CreatedByUserID *int               `gomysql:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time          `gomysql:"created_at,notnull" json:"created_at"`
	}

	QuotaPolicy struct {
		ID            int       `gomysql:"id,primary,increment" json:"id"`
		Name          string    `gomysql:"name,notnull" json:"name"`
		Description   string    `gomysql:"description" json:"description"`
		MaxVMs        *int      `gomysql:"max_vms" json:"max_vms,omitempty"`
		MaxContainers *int      `gomysql:"max_containers" json:"max_containers,omitempty"`
		MaxVCPU       *int      `gomysql:"max_vcpu" json:"max_vcpu,omitempty"`
		MaxMemoryMB   *int      `gomysql:"max_memory_mb" json:"max_memory_mb,omitempty"`
		MaxStorageGB  *int      `gomysql:"max_storage_gb" json:"max_storage_gb,omitempty"`
		MaxNetworks   *int      `gomysql:"max_networks" json:"max_networks,omitempty"`
		MaxPublicIPs  *int      `gomysql:"max_public_ips" json:"max_public_ips,omitempty"`
		CreatedAt     time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	QuotaBinding struct {
		ID            int              `gomysql:"id,primary,increment" json:"id"`
		QuotaPolicyID int              `gomysql:"quota_policy_id,fkey:QuotaPolicy.id,notnull" json:"quota_policy_id"`
		SubjectType   RoleBindingScope `gomysql:"subject_type,notnull" json:"subject_type"`
		SubjectID     int              `gomysql:"subject_id,notnull" json:"subject_id"`
		CreatedAt     time.Time        `gomysql:"created_at,notnull" json:"created_at"`
	}

	Resource struct {
		ID              int            `gomysql:"id,primary,increment" json:"id"`
		UUID            string         `gomysql:"uuid,unique,notnull" json:"uuid"`
		ProjectID       int            `gomysql:"project_id,fkey:Project.id,notnull" json:"project_id"`
		OwnerType       OwnerType      `gomysql:"owner_type,notnull" json:"owner_type"`
		OwnerID         int            `gomysql:"owner_id,notnull" json:"owner_id"`
		ResourceType    ResourceType   `gomysql:"resource_type,notnull" json:"resource_type"`
		Name            string         `gomysql:"name,notnull" json:"name"`
		Slug            string         `gomysql:"slug" json:"slug"`
		Status          ResourceStatus `gomysql:"status,notnull" json:"status"`
		CreatedByUserID *int           `gomysql:"created_by_user_id" json:"created_by_user_id,omitempty"`
		CreatedAt       time.Time      `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt       time.Time      `gomysql:"updated_at,notnull" json:"updated_at"`
		DeletedAt       *time.Time     `gomysql:"deleted_at" json:"deleted_at,omitempty"`
	}

	ProxmoxCluster struct {
		ID                 int       `gomysql:"id,primary,increment" json:"id"`
		UUID               string    `gomysql:"uuid,unique,notnull" json:"uuid"`
		Name               string    `gomysql:"name,notnull" json:"name"`
		APIURL             string    `gomysql:"api_url,notnull" json:"api_url"`
		VerifyTLS          bool      `gomysql:"verify_tls,notnull" json:"verify_tls"`
		CredentialSecretID *int      `gomysql:"credential_secret_id" json:"credential_secret_id,omitempty"`
		IsActive           bool      `gomysql:"is_active,notnull" json:"is_active"`
		CreatedAt          time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt          time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	ProxmoxNode struct {
		ID            int       `gomysql:"id,primary,increment" json:"id"`
		ClusterID     int       `gomysql:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		Name          string    `gomysql:"name,notnull" json:"name"`
		Status        string    `gomysql:"status" json:"status"`
		CPUTotal      int       `gomysql:"cpu_total" json:"cpu_total"`
		MemoryTotalMB int       `gomysql:"memory_total_mb" json:"memory_total_mb"`
		CreatedAt     time.Time `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt     time.Time `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	VirtualMachine struct {
		ID          int        `gomysql:"id,primary,increment" json:"id"`
		ResourceID  int        `gomysql:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID   int        `gomysql:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		NodeID      *int       `gomysql:"node_id,fkey:ProxmoxNode.id" json:"node_id,omitempty"`
		ProxmoxVMID int        `gomysql:"proxmox_vmid,notnull" json:"proxmox_vmid"`
		Name        string     `gomysql:"name,notnull" json:"name"`
		CPUCores    int        `gomysql:"cpu_cores,notnull" json:"cpu_cores"`
		MemoryMB    int        `gomysql:"memory_mb,notnull" json:"memory_mb"`
		DiskGB      *int       `gomysql:"disk_gb" json:"disk_gb,omitempty"`
		OSType      string     `gomysql:"os_type" json:"os_type"`
		TemplateID  *int       `gomysql:"template_id" json:"template_id,omitempty"`
		PowerState  PowerState `gomysql:"power_state,notnull" json:"power_state"`
		CreatedAt   time.Time  `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	Container struct {
		ID          int        `gomysql:"id,primary,increment" json:"id"`
		ResourceID  int        `gomysql:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID   int        `gomysql:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		NodeID      *int       `gomysql:"node_id,fkey:ProxmoxNode.id" json:"node_id,omitempty"`
		ProxmoxVMID int        `gomysql:"proxmox_vmid,notnull" json:"proxmox_vmid"`
		Name        string     `gomysql:"name,notnull" json:"name"`
		CPUCores    int        `gomysql:"cpu_cores,notnull" json:"cpu_cores"`
		MemoryMB    int        `gomysql:"memory_mb,notnull" json:"memory_mb"`
		DiskGB      *int       `gomysql:"disk_gb" json:"disk_gb,omitempty"`
		TemplateID  *int       `gomysql:"template_id" json:"template_id,omitempty"`
		PowerState  PowerState `gomysql:"power_state,notnull" json:"power_state"`
		CreatedAt   time.Time  `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	VirtualNetwork struct {
		ID                 int         `gomysql:"id,primary,increment" json:"id"`
		ResourceID         int         `gomysql:"resource_id,fkey:Resource.id,unique,notnull" json:"resource_id"`
		ClusterID          int         `gomysql:"cluster_id,fkey:ProxmoxCluster.id,notnull" json:"cluster_id"`
		Name               string      `gomysql:"name,notnull" json:"name"`
		NetworkType        NetworkType `gomysql:"network_type,notnull" json:"network_type"`
		CIDRIPv4           string      `gomysql:"cidr_ipv4" json:"cidr_ipv4"`
		CIDRIPv6           string      `gomysql:"cidr_ipv6" json:"cidr_ipv6"`
		VLANID             *int        `gomysql:"vlan_id" json:"vlan_id,omitempty"`
		VXLANID            *int        `gomysql:"vxlan_id" json:"vxlan_id,omitempty"`
		GatewayIPv4        string      `gomysql:"gateway_ipv4" json:"gateway_ipv4"`
		GatewayIPv6        string      `gomysql:"gateway_ipv6" json:"gateway_ipv6"`
		IsInternetRoutable bool        `gomysql:"is_internet_routable,notnull" json:"is_internet_routable"`
		CreatedAt          time.Time   `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt          time.Time   `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	Job struct {
		ID                int        `gomysql:"id,primary,increment" json:"id"`
		UUID              string     `gomysql:"uuid,unique,notnull" json:"uuid"`
		JobType           JobType    `gomysql:"job_type,notnull" json:"job_type"`
		Status            JobStatus  `gomysql:"status,notnull" json:"status"`
		RequestedByUserID *int       `gomysql:"requested_by_user_id" json:"requested_by_user_id,omitempty"`
		ProjectID         *int       `gomysql:"project_id,fkey:Project.id" json:"project_id,omitempty"`
		ResourceID        *int       `gomysql:"resource_id,fkey:Resource.id" json:"resource_id,omitempty"`
		QueueID           string     `gomysql:"queue_id" json:"queue_id"`
		StartedAt         *time.Time `gomysql:"started_at" json:"started_at,omitempty"`
		FinishedAt        *time.Time `gomysql:"finished_at" json:"finished_at,omitempty"`
		CreatedAt         time.Time  `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt         time.Time  `gomysql:"updated_at,notnull" json:"updated_at"`
	}

	JobLog struct {
		ID        int          `gomysql:"id,primary,increment" json:"id"`
		JobID     int          `gomysql:"job_id,fkey:Job.id,notnull" json:"job_id"`
		Stream    JobLogStream `gomysql:"stream,notnull" json:"stream"`
		Message   string       `gomysql:"message,notnull" json:"message"`
		CreatedAt time.Time    `gomysql:"created_at,notnull" json:"created_at"`
	}

	AuditEvent struct {
		ID           int       `gomysql:"id,primary,increment" json:"id"`
		UUID         string    `gomysql:"uuid,unique,notnull" json:"uuid"`
		ActorUserID  *int      `gomysql:"actor_user_id" json:"actor_user_id,omitempty"`
		Action       string    `gomysql:"action,notnull" json:"action"`
		TargetType   string    `gomysql:"target_type" json:"target_type"`
		TargetID     *int      `gomysql:"target_id" json:"target_id,omitempty"`
		ProjectID    *int      `gomysql:"project_id,fkey:Project.id" json:"project_id,omitempty"`
		SourceIP     string    `gomysql:"source_ip" json:"source_ip"`
		UserAgent    string    `gomysql:"user_agent" json:"user_agent"`
		MetadataJSON string    `gomysql:"metadata_json" json:"metadata_json"`
		CreatedAt    time.Time `gomysql:"created_at,notnull" json:"created_at"`
	}

	Secret struct {
		ID             int             `gomysql:"id,primary,increment" json:"id"`
		UUID           string          `gomysql:"uuid,unique,notnull" json:"uuid"`
		Name           string          `gomysql:"name,notnull" json:"name"`
		SecretType     SecretType      `gomysql:"secret_type,notnull" json:"secret_type"`
		EncryptedValue []byte          `gomysql:"encrypted_value,notnull" json:"-"`
		OwnerType      SecretOwnerType `gomysql:"owner_type,notnull" json:"owner_type"`
		OwnerID        *int            `gomysql:"owner_id" json:"owner_id,omitempty"`
		CreatedAt      time.Time       `gomysql:"created_at,notnull" json:"created_at"`
		UpdatedAt      time.Time       `gomysql:"updated_at,notnull" json:"updated_at"`
	}
)
