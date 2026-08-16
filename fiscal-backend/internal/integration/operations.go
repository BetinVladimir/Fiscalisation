package integration

import "context"

func (s *Service) IntegrationMetrics(ctx context.Context) (map[string]int64, error) {
	queries := map[string]string{
		"command_backlog":      `select count(*) from integration_commands where status in ('ACCEPTED','QUEUED','PROCESSING')`,
		"command_dead":         `select count(*) from integration_commands where status='DEAD'`,
		"webhook_backlog":      `select count(*) from webhook_deliveries where status in ('PENDING','LEASED','QUEUED','DELIVERING','RETRY')`,
		"webhook_dead":         `select count(*) from webhook_deliveries where status='DEAD'`,
		"enrollment_conflicts": `select count(*) from external_enrollment_challenges where status='CONFLICT'`,
		"otp_locked":           `select (select count(*) from external_enrollment_challenges where status='LOCKED')+(select count(*) from app_auth_challenges where status='LOCKED')`,
		"security_rejections":  `select count(*) from integration_security_events where occurred_at>now()-interval '1 hour'`,
		"source_conflicts":     `select count(*) from integration_change_journal where outcome='REJECTED' and occurred_at>now()-interval '1 hour'`,
		"command_oldest_sec":   `select coalesce(extract(epoch from now()-min(created_at))::bigint,0) from integration_commands where status in ('ACCEPTED','QUEUED','PROCESSING')`,
		"webhook_oldest_sec":   `select coalesce(extract(epoch from now()-min(created_at))::bigint,0) from webhook_deliveries where status in ('PENDING','LEASED','QUEUED','DELIVERING','RETRY')`,
	}
	out := map[string]int64{}
	for name, q := range queries {
		var n int64
		if e := s.db.QueryRowContext(ctx, q).Scan(&n); e != nil {
			return nil, e
		}
		out[name] = n
	}
	return out, nil
}
