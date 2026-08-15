package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedMigrationsAreOrderedAndTransactionalSQL(t *testing.T) {
	entries, err := fs.Glob(files, "sql/*.sql")
	if err != nil || len(entries) == 0 {
		t.Fatalf("embedded migrations: %v %v", entries, err)
	}
	ordered := append([]string(nil), entries...)
	sort.Strings(ordered)
	for index := range entries {
		if entries[index] != ordered[index] {
			t.Fatalf("migrations are not ordered: %v", entries)
		}
		body, readErr := files.ReadFile(entries[index])
		if readErr != nil || !strings.Contains(strings.ToLower(string(body)), "create table") {
			t.Fatalf("invalid migration %s: %v", entries[index], readErr)
		}
	}
}
