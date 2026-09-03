package auth

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/internal/safefile"
	"github.com/go-ldap/ldap/v3"
)

const ldapUserDNFormat = "uid=%s,cn=%s,cn=accounts,dc=%s,dc=%s"

var ErrUnauthorized error = errors.New("unauthorized")

func getUsername(s string) (valueResult string) {
	return fmt.Sprintf(
		ldapUserDNFormat,
		ldap.EscapeDN(s),
		ldap.EscapeDN(config.Config.LDAP.UsersCN),
		ldap.EscapeDN(config.Config.LDAP.DomainSLD),
		ldap.EscapeDN(config.Config.LDAP.DomainTLD),
	)
}

func getGroupName(s string) (valueResult string) {
	return fmt.Sprintf(
		"cn=%s,cn=%s,cn=accounts,dc=%s,dc=%s",
		ldap.EscapeDN(s),
		ldap.EscapeDN(config.Config.LDAP.GroupsCN),
		ldap.EscapeDN(config.Config.LDAP.DomainSLD),
		ldap.EscapeDN(config.Config.LDAP.DomainTLD),
	)
}

func getFilter() (valueResult string) {
	return fmt.Sprintf(
		"cn=%s,cn=accounts,dc=%s,dc=%s",
		ldap.EscapeDN(config.Config.LDAP.UsersCN),
		ldap.EscapeDN(config.Config.LDAP.DomainSLD),
		ldap.EscapeDN(config.Config.LDAP.DomainTLD),
	)
}

func getGroupsFilter() (valueResult string) {
	return fmt.Sprintf(
		"cn=%s,cn=accounts,dc=%s,dc=%s",
		ldap.EscapeDN(config.Config.LDAP.GroupsCN),
		ldap.EscapeDN(config.Config.LDAP.DomainSLD),
		ldap.EscapeDN(config.Config.LDAP.DomainTLD),
	)
}

// dialLDAP opens an LDAP connection using verified TLS and an optional custom CA bundle.
func dialLDAP() (connResult *ldap.Conn, errResult error) {
	var address *url.URL

	if address, errResult = url.Parse(config.Config.LDAP.Address); errResult != nil {
		errResult = fmt.Errorf("parse LDAP address: %w", errResult)
		return
	}

	if address.Scheme != "ldaps" {
		errResult = fmt.Errorf("LDAP address must use ldaps:// to protect credentials")
		return
	}

	var tlsConfig *tls.Config = &tls.Config{MinVersion: tls.VersionTLS12}

	if config.Config.LDAP.CAFile != "" {
		var (
			certificatePool *x509.CertPool
			certificatePEM  []byte
		)

		if certificatePool, errResult = x509.SystemCertPool(); errResult != nil {
			errResult = fmt.Errorf("load system certificate pool: %w", errResult)
			return
		}

		if certificatePEM, errResult = safefile.Read(config.Config.LDAP.CAFile); errResult != nil {
			errResult = fmt.Errorf("load LDAP CA file: %w", errResult)
			return
		}

		if !certificatePool.AppendCertsFromPEM(certificatePEM) {
			errResult = fmt.Errorf("LDAP CA file contains no valid certificates")
			return
		}

		tlsConfig.RootCAs = certificatePool
	}

	if connResult, errResult = ldap.DialURL(config.Config.LDAP.Address, ldap.DialWithTLSConfig(tlsConfig)); errResult != nil {
		errResult = fmt.Errorf("connect to LDAP: %w", errResult)
	}

	return
}

// UserExists reports whether an LDAP user exists.
func UserExists(username string) (exists bool, err error) {
	var conn *ldap.Conn
	if conn, err = dialLDAP(); err != nil {
		return
	}

	defer func() {
		_ = conn.Close()
	}()

	var result *ldap.SearchResult
	if result, err = conn.Search(ldap.NewSearchRequest(
		getFilter(),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)); err != nil {
		return
	}

	exists = len(result.Entries) > 0
	return
}

type (
	LDAPConn struct {
		conn            *ldap.Conn
		Username        string
		IsAuthenticated bool
	}

	LDAPUser struct {
		Username    string
		DisplayName string
		Email       string
	}

	LDAPGroup struct {
		Name string
	}
)

// LookupUser looks up user.
func LookupUser(username string) (lDAPUserResult *LDAPUser, okResult bool, errResult error) {
	var conn *ldap.Conn
	var err error
	if conn, err = dialLDAP(); err != nil {
		return nil, false, err
	}
	defer func() {
		_ = conn.Close()
	}()

	var result *ldap.SearchResult
	if result, err = conn.Search(ldap.NewSearchRequest(
		getFilter(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(|(uid=%s)(mail=%s)(displayName=%s)(cn=%s))",
			ldap.EscapeFilter(username),
			ldap.EscapeFilter(username),
			ldap.EscapeFilter(username),
			ldap.EscapeFilter(username),
		),
		[]string{"uid", "displayName", "mail", "cn"},
		nil,
	)); err != nil {
		return nil, false, err
	}

	if len(result.Entries) == 0 {
		return nil, false, nil
	}
	var entry *ldap.Entry

	entry = result.Entries[0]
	var displayName string

	displayName = entry.GetAttributeValue("displayName")
	if displayName == "" {
		displayName = entry.GetAttributeValue("cn")
	}
	var user *LDAPUser

	user = &LDAPUser{
		Username:    entry.GetAttributeValue("uid"),
		DisplayName: displayName,
		Email:       entry.GetAttributeValue("mail"),
	}
	if user.Username == "" {
		user.Username = username
	}
	return user, true, nil
}

// LookupGroup looks up group.
func LookupGroup(name string) (lDAPGroupResult *LDAPGroup, okResult bool, errResult error) {
	var conn *ldap.Conn
	var err error
	if conn, err = dialLDAP(); err != nil {
		return nil, false, err
	}
	defer func() {
		_ = conn.Close()
	}()

	var result *ldap.SearchResult
	if result, err = conn.Search(ldap.NewSearchRequest(
		getGroupsFilter(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(cn=%s)", ldap.EscapeFilter(name)),
		[]string{"cn"},
		nil,
	)); err != nil {
		return nil, false, err
	}

	if len(result.Entries) == 0 {
		return nil, false, nil
	}
	var group *LDAPGroup

	group = &LDAPGroup{Name: result.Entries[0].GetAttributeValue("cn")}
	if group.Name == "" {
		group.Name = name
	}
	return group, true, nil
}

// NewLDAPConn opens and binds an LDAP connection for a user.
func NewLDAPConn(username, password string) (conn *LDAPConn, err error) {
	var socket *ldap.Conn
	if socket, err = dialLDAP(); err != nil {
		return
	}

	conn = &LDAPConn{
		conn:            socket,
		Username:        username,
		IsAuthenticated: socket.Bind(getUsername(username), password) == nil,
	}

	return
}

// Close releases the underlying LDAP connection.
func (l *LDAPConn) Close() (errResult error) {
	if l.conn != nil {
		errResult = l.conn.Close()
	}

	return
}

// WhoAmI returns the LDAP authorization identity for the connection.
func (l *LDAPConn) WhoAmI() (id string, err error) {
	if !l.IsAuthenticated {
		err = ErrUnauthorized
		return
	}

	var who *ldap.WhoAmIResult
	if who, err = l.conn.WhoAmI(nil); err != nil {
		return
	}

	id = who.AuthzID
	return
}

// Groups returns LDAP group names for the authenticated user.
func (l *LDAPConn) Groups() (groups []string, err error) {
	if !l.IsAuthenticated {
		err = ErrUnauthorized
		return
	}

	var result *ldap.SearchResult
	if result, err = l.conn.Search(ldap.NewSearchRequest(
		getGroupsFilter(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(member=%s))", getUsername(l.Username)),
		[]string{"cn"}, nil,
	)); err != nil {
		return nil, err
	}

	for _, entry := range result.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}

	return
}

// GetAttributes returns selected LDAP attributes for the authenticated user.
func (l *LDAPConn) GetAttributes(attrs ...string) (attributes map[string]string, err error) {
	if !l.IsAuthenticated {
		err = ErrUnauthorized
		return
	}

	var result *ldap.SearchResult
	if result, err = l.conn.Search(ldap.NewSearchRequest(
		getFilter(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(l.Username)), attrs, nil,
	)); err != nil {
		return
	}

	if len(result.Entries) == 0 {
		err = fmt.Errorf("no entries found")
		return
	}

	var entry *ldap.Entry = result.Entries[0]
	attributes = make(map[string]string)
	for _, attr := range attrs {
		attributes[attr] = entry.GetAttributeValue(attr)
	}

	return
}

// IsMemberOf reports whether the authenticated user belongs to a group.
func (l *LDAPConn) IsMemberOf(groupName string) (isMember bool, err error) {
	if !l.IsAuthenticated {
		err = ErrUnauthorized
		return
	}

	var result *ldap.SearchResult
	if result, err = l.conn.Search(ldap.NewSearchRequest(
		getGroupName(groupName), ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 0, false,
		fmt.Sprintf("(member=%s)", getUsername(l.Username)), []string{"cn"}, nil,
	)); err != nil {
		return false, err
	}

	isMember = len(result.Entries) > 0
	return
}

// DisplayName returns the LDAP display name for the authenticated user.
func (l *LDAPConn) DisplayName() (displayName string, err error) {
	var attributes map[string]string
	if attributes, err = l.GetAttributes("displayName"); err == nil {
		displayName = attributes["displayName"]
	}

	return
}

// Email returns the LDAP email address for the authenticated user.
func (l *LDAPConn) Email() (email string, err error) {
	var attributes map[string]string
	if attributes, err = l.GetAttributes("mail"); err == nil {
		email = attributes["mail"]
	}

	return
}

// UID returns the numeric LDAP uid for the authenticated user.
func (l *LDAPConn) UID() (uid uint64, err error) {
	var attributes map[string]string
	if attributes, err = l.GetAttributes("uidNumber"); err == nil {
		_, err = fmt.Sscanf(attributes["uidNumber"], "%d", &uid)
	}

	return
}
