package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGeneratesDefaultConfigWhenMissing(t *testing.T) {
	Config = Configuration{}
	var path string

	path = filepath.Join(t.TempDir(), "config.toml")
	var err error

	err = Init(path)
	if err == nil {
		t.Fatal("expected Init to report that a default config was created")
	}

	if !strings.Contains(err.Error(), "created a default config") {
		t.Fatalf("expected default-config error, got %v", err)
	}
	{
		var statErr error

		if _, statErr = os.Stat(path); statErr != nil {
			t.Fatalf("expected generated config at %s: %v", path, statErr)
		}
	}
}

func TestInitLoadsValidConfig(t *testing.T) {
	Config = Configuration{}
	var path string

	path = filepath.Join(t.TempDir(), "config.toml")
	var content string

	content = `
[web_server]
address = ":0"

[ldap]
address = "ldaps://ldap.example.test:636"
domain_sld = "example"
domain_tld = "test"
accounts_cn = "accounts"
users_cn = "users"
groups_cn = "groups"
admin_groups = ["admins"]
user_groups = ["ipausers"]

[database]
file = "test.db"

[proxmox]
enabled = false
`
	{
		var err error

		if err = os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	{
		var err error

		if err = Init(path); err != nil {
			t.Fatalf("Init returned error: %v", err)
		}
	}

	if Config.WebServer.Address != ":0" {
		t.Fatalf("expected web address :0, got %q", Config.WebServer.Address)
	}
	if Config.LDAP.DomainSLD != "example" {
		t.Fatalf("expected LDAP domain_sld example, got %q", Config.LDAP.DomainSLD)
	}
	if Config.Database.File != "test.db" {
		t.Fatalf("expected database file test.db, got %q", Config.Database.File)
	}
}
