package db

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentQuotaReservationsCannotExceedProjectLimit(t *testing.T) {
	initTestDB(t)
	var now time.Time = time.Now().UTC()
	var org *Organization = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var limit int = 8
	var (
		policy *QuotaPolicy
		err    error
	)

	policy, err = CreateQuotaPolicy(QuotaPolicyInput{Name: "One lab", MaxVMs: &limit})
	if err != nil {
		t.Fatalf("CreateQuotaPolicy returned error: %v", err)
	}
	if _, err = BindQuotaPolicy(policy.ID, RoleBindingScopeProject, project.ID); err != nil {
		t.Fatalf("BindQuotaPolicy returned error: %v", err)
	}
	var (
		waitGroup sync.WaitGroup
		results   chan error = make(chan error, 2)
	)

	for count := 0; count < 2; count++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			var reserveErr error

			_, reserveErr = ReserveProjectQuota(QuotaReservationInput{ProjectID: project.ID, Dimensions: QuotaDimensions{VMs: 8}})
			results <- reserveErr
		}()
	}
	waitGroup.Wait()
	close(results)
	var successes, failures int
	for result := range results {
		if result == nil {
			successes++
		} else if strings.Contains(result.Error(), "quota exceeded") {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected one reservation and one quota failure, successes=%d failures=%d", successes, failures)
	}
}

func TestSecretLifecycleEncryptsAtRestAndRedactsAuditMetadata(t *testing.T) {
	initTestDB(t)
	var key []byte = []byte("01234567890123456789012345678901")
	var err error
	if err = ConfigureSecretEncryption(base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatalf("ConfigureSecretEncryption returned error: %v", err)
	}
	var secret *Secret
	secret, err = CreateSecret(SecretCreateInput{Name: "cluster token", SecretType: SecretTypeProxmoxToken, Value: "super-secret-value", OwnerType: SecretOwnerTypeSystem})
	if err != nil {
		t.Fatalf("CreateSecret returned error: %v", err)
	}
	if strings.Contains(string(secret.EncryptedValue), "super-secret-value") {
		t.Fatal("plaintext secret was stored")
	}
	var plaintext string
	plaintext, err = ReadSecret(secret)
	if err != nil || plaintext != "super-secret-value" {
		t.Fatalf("ReadSecret returned value=%q err=%v", plaintext, err)
	}
	if err = RotateSecret(secret, "rotated-secret"); err != nil {
		t.Fatalf("RotateSecret returned error: %v", err)
	}
	var event *AuditEvent
	event, err = WriteAudit(AuditInput{Action: "secret.rotate", TargetType: "secret", TargetID: &secret.ID, Metadata: map[string]any{"token": "rotated-secret", "safe": "retained"}})
	if err != nil {
		t.Fatalf("WriteAudit returned error: %v", err)
	}
	if strings.Contains(event.MetadataJSON, "rotated-secret") || !strings.Contains(event.MetadataJSON, "[REDACTED]") {
		t.Fatalf("audit metadata was not redacted: %s", event.MetadataJSON)
	}
	if err = ArchiveSecret(secret); err != nil {
		t.Fatalf("ArchiveSecret returned error: %v", err)
	}
	if len(secret.EncryptedValue) != 0 || secret.ArchivedAt == nil {
		t.Fatalf("expected archived ciphertext to be cleared, got %#v", secret)
	}
}

func TestSecretCreationRequiresConfiguredEncryption(t *testing.T) {
	initTestDB(t)
	var err error

	if _, err = CreateSecret(SecretCreateInput{Name: "unsafe", Value: "plaintext"}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing encryption configuration error, got %v", err)
	}
}

func TestArchiveProjectPreservesRelatedHistory(t *testing.T) {
	initTestDB(t)
	var now time.Time = time.Now().UTC()
	var org *Organization = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var resource *Resource = insertTestResource(t, project.ID, "student-vm", now)
	var job *Job
	var err error
	job, err = CreateJob(JobCreateInput{JobType: JobTypeCleanup, ProjectID: &project.ID, ResourceID: &resource.ID})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if _, err = WriteAudit(AuditInput{Action: "job.create", TargetType: "job", TargetID: &job.ID, ProjectID: &project.ID}); err != nil {
		t.Fatalf("WriteAudit returned error: %v", err)
	}
	if err = ArchiveProject(project); err != nil {
		t.Fatalf("ArchiveProject returned error: %v", err)
	}
	if project.IsActive {
		t.Fatal("expected project to be inactive")
	}
	var (
		storedResource *Resource
		storedJob      *Job
		selectErr      error
	)

	if storedResource, selectErr = Resources.Select(resource.ID); selectErr != nil || storedResource == nil {
		t.Fatalf("expected resource preserved, resource=%#v err=%v", storedResource, selectErr)
	}
	if storedJob, selectErr = Jobs.Select(job.ID); selectErr != nil || storedJob == nil {
		t.Fatalf("expected job preserved, job=%#v err=%v", storedJob, selectErr)
	}
	var events []*AuditEvent
	events, err = AuditEventsForProject(&project.ID)
	if err != nil || len(events) < 1 {
		t.Fatalf("expected audit history preserved, events=%#v err=%v", events, err)
	}
}
