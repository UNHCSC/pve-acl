package app

import (
	"reflect"
	"testing"
)

func TestUserImportQueriesDeduplicatesEntries(t *testing.T) {
	got := userImportQueries(userImportRequest{
		Entries:   "alice, bob\nAlice\tcarol@example.test",
		Usernames: []string{"dave", "bob"},
	})
	want := []string{"dave", "bob", "alice", "carol@example.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userImportQueries() = %#v, want %#v", got, want)
	}
}
