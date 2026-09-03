package proxmox

import "testing"

func TestParseTagsAndExactManagedMatch(t *testing.T) {
	var guest Guest = Guest{Tags: ParseTags("student; organesson-managed ;student,organesson-managed-extra")}
	if !HasTag(guest, DefaultManagedTag) {
		t.Fatal("expected exact managed tag to match")
	}
	if HasTag(Guest{Tags: []string{"organesson-managed-extra"}}, DefaultManagedTag) {
		t.Fatal("expected tag prefix not to match")
	}
	if len(guest.Tags) != 3 {
		t.Fatalf("expected normalized unique tags, got %#v", guest.Tags)
	}
}
