package journal

import "testing"

func TestStorageThresholds(t *testing.T) {
	tests := []struct {
		used int64
		want string
	}{{0, "NORMAL"}, {699, "NORMAL"}, {700, "WARNING"}, {850, "HIGH"}, {950, "CRITICAL"}, {1000, "FULL"}, {1200, "FULL"}}
	for _, test := range tests {
		if got := ClassifyStorage(test.used, 1000); got.State != test.want {
			t.Fatalf("used=%d: want %s got %s (%d%%)", test.used, test.want, got.State, got.Percent)
		}
	}
	if got := ClassifyStorage(1, 0); got.State != "UNBOUNDED" {
		t.Fatalf("unexpected unbounded state: %+v", got)
	}
}

func TestStorageUsesAllocatedSQLitePages(t *testing.T) {
	j, err := Open(t.TempDir() + "/edge.db")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	status, err := j.Storage(1024 * 1024)
	if err != nil {
		t.Fatal(err)
	}
	if status.UsedBytes <= 0 || status.QuotaBytes != 1024*1024 {
		t.Fatalf("invalid storage status: %+v", status)
	}
}
