package config

type Configuration struct {
	WebServer struct {
		Address                     string   `toml:"address" default:":8080" validate:"required"`                     // Listen address for the web application server e.g. ":8080", "0.0.0.0:8080"
		TLSDir                      string   `toml:"tls_dir" default:""`                                              // Directory containing a crt and a key file for TLS. Leave empty to use HTTP instead of HTTPS.
		ReloadTemplatesOnEachRender bool     `toml:"reload_templates_on_each_render" default:"false"`                 // For development purposes. If true, templates are reloaded from disk on each render.
		RedirectServerAddresses     []string `toml:"redirect_server_addresses" default:"[]" validate:"dive,required"` // List of addresses ("host:port", or ":port") to which HTTP requests should be redirected to HTTPS. If your web app is on ":443", you might want to redirect ":80" here.
	} `toml:"web_server"` // Web server configuration

	LDAP struct {
		Address     string   `toml:"address" default:"" validate:"required"`                   // LDAP server address (e.g. "ldaps://domain.cyber.lab:636")
		CAFile      string   `toml:"ca_file" default:""`                                       // Optional PEM CA bundle used to verify the LDAP server certificate.
		DomainSLD   string   `toml:"domain_sld" default:"" validate:"required"`                // LDAP domain second-level domain (e.g. "cyber" for "domain.cyber.lab")
		DomainTLD   string   `toml:"domain_tld" default:"" validate:"required"`                // LDAP domain top-level domain (e.g. "lab" for "domain.cyber.lab")
		AccountsCN  string   `toml:"accounts_cn" default:"accounts" validate:"required"`       // LDAP container name for accounts (usually "accounts")
		UsersCN     string   `toml:"users_cn" default:"users" validate:"required"`             // LDAP container name for users (usually "users")
		GroupsCN    string   `toml:"groups_cn" default:"groups" validate:"required"`           // LDAP container name for groups (usually "groups")
		AdminGroups []string `toml:"admin_groups" default:"[\"admins\"]" validate:"required"`  // LDAP groups whose members should have admin access to the web app
		UserGroups  []string `toml:"user_groups" default:"[\"ipausers\"]" validate:"required"` // LDAP groups whose members should have user access to the web app
	} `toml:"ldap"` // LDAP configuration

	Database struct {
		File string `toml:"file" default:"organesson.db" validate:"required"` // Path to the MySQL database file
	} `toml:"database"` // Database configuration

	Scheduler struct {
		DatabaseFile         string `toml:"database_file" default:""` // Path to the gasket task database. Empty uses the application database path with ".tasks" appended.
		GlobalConcurrency    int    `toml:"global_concurrency" default:"4"`
		PerNodeConcurrency   int    `toml:"per_node_concurrency" default:"2"`
		ShutdownDrainSeconds int    `toml:"shutdown_drain_seconds" default:"20"`
	} `toml:"scheduler"` // Embedded task scheduler configuration

	Runner struct {
		WorkDir            string   `toml:"work_dir" default:"runner-data/work"`
		StateDir           string   `toml:"state_dir" default:"runner-data/state"`
		AllowedSourceRoots []string `toml:"allowed_source_roots" default:"[\"examples\"]"`
		OpenTofuExecutable string   `toml:"opentofu_executable" default:"tofu"`
		AnsibleExecutable  string   `toml:"ansible_executable" default:"ansible-playbook"`
		TimeoutSeconds     int      `toml:"timeout_seconds" default:"1800"`
		MaxOutputBytes     int      `toml:"max_output_bytes" default:"1048576"`
	} `toml:"runner"`

	Secrets struct {
		MasterKey string `toml:"master_key" default:""` // Base64-encoded 32-byte application encryption key.
	} `toml:"secrets"`

	Proxmox struct {
		Enabled              bool   `toml:"enabled" default:"false"`                  // Enable Proxmox VE integration.
		Hostname             string `toml:"hostname" default:""`                      // Proxmox VE server hostname, IP address, or HTTPS URL.
		Port                 string `toml:"port" default:"8006"`                      // Proxmox VE API port.
		TokenID              string `toml:"token_id" default:""`                      // Proxmox VE API token ID, including the user realm.
		Secret               string `toml:"secret" default:""`                        // Proxmox VE API token secret.
		VerifyTLS            bool   `toml:"verify_tls" default:"true"`                // Verify the Proxmox API TLS certificate.
		TLSFingerprintSHA256 string `toml:"tls_fingerprint_sha256" default:""`        // Optional SHA-256 certificate pin for a private Proxmox CA.
		ClusterID            string `toml:"cluster_id" default:""`                    // Stable local identity for this cluster; defaults to hostname.
		ManagedTag           string `toml:"managed_tag" default:"organesson-managed"` // Required tag for guests visible to Organesson.
	} `toml:"proxmox"` // Proxmox VE integration configuration
}

var Config Configuration
