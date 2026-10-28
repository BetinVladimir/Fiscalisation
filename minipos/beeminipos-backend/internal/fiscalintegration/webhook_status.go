package fiscalintegration

import "fmt"

// ErrUnknownWebhookStatus is returned when the webhook payload carries a status
// string that is not in the known Fiscal operation state set.
type ErrUnknownWebhookStatus struct{ Status string }

func (e *ErrUnknownWebhookStatus) Error() string {
	return fmt.Sprintf("UNKNOWN_WEBHOOK_STATUS: %s", e.Status)
}

// mapWebhookStatus maps a Fiscal webhook status string to the sync_status value
// that should be stored in company_fiscal_resource_links.
//
// Terminal states:      SUCCEEDED, FAILED (→ FAILED)
// Non-terminal states:  ACCEPTED, IN_PROGRESS, RETRYING (→ ACCEPTED, no change)
// Unknown:              returns ErrUnknownWebhookStatus
func mapWebhookStatus(status string) (string, error) {
	switch status {
	case "SUCCEEDED":
		return "SUCCEEDED", nil
	case "SUPERSEDED":
		return "SUPERSEDED", nil
	case "FAILED", "REJECTED":
		return "FAILED", nil
	case "ACCEPTED", "IN_PROGRESS", "RETRYING":
		return "ACCEPTED", nil
	default:
		return "", &ErrUnknownWebhookStatus{Status: status}
	}
}
