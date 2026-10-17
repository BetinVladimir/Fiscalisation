package domain

import "errors"

// BuildFinalizePayload is the single immutable monetary plan used by both
// authenticated MQTT commands and root-signed local online authorization.
func BuildFinalizePayload(sale Sale, operatorID, operatorCode string) (map[string]any, error) {
	if sale.ID == "" || operatorID == "" || !bgOperatorPattern.MatchString(operatorCode) {
		return nil, errors.New("finalize identity missing")
	}
	unp, err := ParseBGUNP(sale.UNP)
	if err != nil || unp.OperatorCode != operatorCode {
		return nil, errors.New("server assigned UNP missing or inconsistent")
	}
	payments := make([]PaymentRequest, 0, len(sale.Payments))
	for _, p := range sale.Payments {
		payments = append(payments, PaymentRequest{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount})
	}
	payload := map[string]any{"currency": "EUR", "server_sale_id": sale.ID, "external_id": sale.ExternalID, "location_id": sale.LocationID, "operator_id": operatorID, "operator_code": operatorCode, "items": sale.Lines, "payments": payments, "metadata": map[string]any{}, "unp_source": "SERVER_ASSIGNED", "unp": sale.UNP}
	return edgeDeepMap(payload), nil
}
