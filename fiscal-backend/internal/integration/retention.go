package integration

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

func (s *Service) runRetentionBatch(ctx context.Context) (int64, error) {
	started := s.now()
	rows, err := s.db.QueryContext(ctx, `select kind,moved from archive_integration_operational_rows(1000)`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	moved := map[string]int64{}
	var total int64
	for rows.Next() {
		var kind string
		var count int64
		if err = rows.Scan(&kind, &count); err != nil {
			return 0, err
		}
		moved[kind] = count
		total += count
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	payload, _ := json.Marshal(moved)
	_, err = s.db.ExecContext(ctx, `insert into integration_retention_runs(started_at,moved) values($1,$2)`, started, payload)
	return total, err
}

// RunRetentionWorker archives terminal operational rows before removing them
// from hot tables. Append-only integration and security journals are excluded.
func (s *Service) RunRetentionWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		// Drain bounded batches so a high-volume installation catches up without
		// turning retention into an unbounded database job.
		for batch := 0; batch < 100; batch++ {
			moved, err := s.runRetentionBatch(ctx)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("integration retention failed: %v", err)
				}
				break
			}
			if moved == 0 {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
