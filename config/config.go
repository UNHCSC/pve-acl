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
		DomainSLD   string   `toml:"domain_sld" default:"" validate:"required"`                // LDAP domain second-level domain (e.g. "cyber" for "domain.cyber.lab")
		DomainTLD   string   `toml:"domain_tld" default:"" validate:"required"`                // LDAP domain top-level domain (e.g. "lab" for "domain.cyber.lab")
		AccountsCN  string   `toml:"accounts_cn" default:"accounts" validate:"required"`       // LDAP container name for accounts (usually "accounts")
		UsersCN     string   `toml:"users_cn" default:"users" validate:"required"`             // LDAP container name for users (usually "users")
		GroupsCN    string   `toml:"groups_cn" default:"groups" validate:"required"`           // LDAP container name for groups (usually "groups")
		AdminGroups []string `toml:"admin_groups" default:"[\"admins\"]" validate:"required"`  // LDAP groups whose members should have admin access to the web app
		UserGroups  []string `toml:"user_groups" default:"[\"ipausers\"]" validate:"required"` // LDAP groups whose members should have user access to the web app
	} `toml:"ldap"` // LDAP configuration

	Database struct {
		File string `toml:"file" default:"pve-acl.db" validate:"required"` // Path to the MySQL database file
	} `toml:"database"` // Database configuration

	Proxmox struct {
		Enabled  bool   `toml:"enabled" default:"false"`           // Enable Proxmox VE integration
		Hostname string `toml:"hostname" default:""`               // Proxmox VE server hostname or IP address (e.g. "proxmox.cyber.lab")
		Port     string `toml:"port" default:""`                   // Proxmox VE API port (usually "8006")
		TokenID  string `toml:"token_id" default:""`               // Proxmox VE API token ID (e.g. "api-token-id")
		Secret   string `toml:"secret" default:"api-token-secret"` // Proxmox VE API token secret
	} `toml:"proxmox"` // Proxmox VE integration configuration
}

var Config Configuration
