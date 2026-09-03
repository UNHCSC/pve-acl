package db

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/z46-dev/gosqlite"
)

type AuditInput struct {
	ActorUserID *int
	Action      string
	TargetType  string
	TargetID    *int
	ProjectID   *int
	SourceIP    string
	UserAgent   string
	Metadata    map[string]any
}

// WriteAudit records a sanitized immutable audit event.
func WriteAudit(input AuditInput) (eventResult *AuditEvent, errResult error) {
	var (
		uuid     string
		metadata []byte
	)

	uuid, errResult = randomUUID()
	if errResult != nil {
		return nil, errResult
	}
	metadata, errResult = json.Marshal(redactAuditMap(input.Metadata))
	if errResult != nil {
		return nil, errResult
	}
	var event *AuditEvent

	event = &AuditEvent{UUID: uuid, ActorUserID: input.ActorUserID, Action: strings.TrimSpace(input.Action), TargetType: strings.TrimSpace(input.TargetType), TargetID: input.TargetID, ProjectID: input.ProjectID, SourceIP: strings.TrimSpace(input.SourceIP), UserAgent: strings.TrimSpace(input.UserAgent), MetadataJSON: string(metadata), CreatedAt: time.Now().UTC()}
	if errResult = AuditEvents.Insert(event); errResult != nil {
		return nil, errResult
	}
	return event, nil
}

// AuditEventsForProject lists global or project-specific audit history.
func AuditEventsForProject(projectID *int) (itemsResult []*AuditEvent, errResult error) {
	if projectID == nil {
		return AuditEvents.SelectAll()
	}
	return AuditEvents.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(AuditEvents.FieldBySQLName("project_id"), gosqlite.OpEqual, *projectID).Limit(1000))
}

func redactAuditMap(values map[string]any) (result map[string]any) {
	result = make(map[string]any, len(values))
	for key, value := range values {
		var lower string = strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "credential") || lower == "value" {
			result[key] = "[REDACTED]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			result[key] = redactAuditMap(nested)
		} else {
			result[key] = value
		}
	}
	return
}
