package persistence

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Keep a public whitelist, never merge the full durable payload: it contains
// internal execution authorization identifiers and may contain legacy metadata.
// The payload is the durable authority for prepared fields with no typed column.
const typedOrderSelect = `select jsonb_build_object('id',id,'tenant_id',organization_id,'external_id',external_id,'shift_id',shift_id,'register_id',register_id,'operator_code',operator_code,'state',state,'total',jsonb_build_object('amount',total::text,'currency',currency),'lines',lines,'payments',payments,'fiscal_sale_id',coalesce(fiscal_sale_id,''),'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_reference',coalesce(fiscal_reference,''),'reversal_operation_id',coalesce(reversal_operation_id,''),'reversal_fiscal_reference',coalesce(reversal_fiscal_reference,''),'reversal_reason_code',coalesce(reversal_reason_code,''),'fiscal_version',coalesce(fiscal_version,0),'version',version,'created_at',created_at,'updated_at',updated_at),payload from minipos.minipos_runtime_orders`

func orderObject(raw []byte) (map[string]any, error) {
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&v) != nil || v == nil {
		return nil, errors.New("invalid persisted order projection")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("invalid persisted order projection")
	}
	return v, nil
}
func projectedOrder(raw, payload []byte) ([]byte, error) {
	typed, err := orderObject(raw)
	if err != nil {
		return nil, err
	}
	source, err := orderObject(payload)
	if err != nil {
		return nil, err
	}
	conflict := func() ([]byte, error) { return nil, errors.New("persisted order projection conflict") }
	for _, key := range []string{"id", "tenant_id", "external_id", "shift_id", "register_id", "operator_code", "state", "version", "total", "lines"} {
		if !reflect.DeepEqual(typed[key], source[key]) {
			return conflict()
		}
	}
	for _, key := range []string{"fiscal_sale_id", "fiscal_operation_id", "receipt_reference", "reversal_operation_id", "reversal_fiscal_reference", "reversal_reason_code"} {
		v, ok := source[key]
		if !ok {
			v = ""
		}
		if !reflect.DeepEqual(typed[key], v) {
			return conflict()
		}
	}
	version, ok := source["fiscal_version"]
	if !ok {
		version = json.Number("0")
	}
	if !reflect.DeepEqual(typed["fiscal_version"], version) {
		return conflict()
	}
	payments, ok := source["payments"]
	if !ok || payments == nil {
		payments = []any{}
	}
	if !reflect.DeepEqual(typed["payments"], payments) {
		return conflict()
	}
	for _, key := range []string{"checkout_operation_id", "receipt_session_id", "fiscal_register_id"} {
		v, exists := source[key]
		if !exists {
			continue
		}
		text, valid := v.(string)
		if !valid || strings.TrimSpace(text) != text {
			return conflict()
		}
		typed[key] = text
	}
	checkout, _ := typed["checkout_operation_id"].(string)
	operation, _ := typed["fiscal_operation_id"].(string)
	if checkout != "" && operation != "" && checkout != operation {
		return conflict()
	}
	// Even synchronized legacy payload/typed payment metadata must not escape a
	// public entity read. Domain confirmation hashes only these prepared fields.
	values, valid := typed["payments"].([]any)
	if !valid {
		return conflict()
	}
	public := make([]any, 0, len(values))
	for _, value := range values {
		payment, ok := value.(map[string]any)
		if !ok {
			return conflict()
		}
		entry := map[string]any{}
		for _, key := range []string{"payment_id", "type", "terminal_policy"} {
			if v, exists := payment[key]; exists {
				if _, ok := v.(string); !ok {
					return conflict()
				}
				entry[key] = v
			}
		}
		money, ok := payment["amount"].(map[string]any)
		if !ok {
			return conflict()
		}
		amount, a := money["amount"].(string)
		currency, c := money["currency"].(string)
		if !a || !c {
			return conflict()
		}
		entry["amount"] = map[string]any{"amount": amount, "currency": currency}
		public = append(public, entry)
	}
	lines, valid := typed["lines"].([]any)
	if !valid {
		return conflict()
	}
	safeLines := make([]any, 0, len(lines))
	for _, value := range lines {
		line, ok := value.(map[string]any)
		if !ok {
			return conflict()
		}
		entry := map[string]any{}
		for _, key := range []string{"line_id", "product_id", "name", "quantity", "tax_group"} {
			if v, exists := line[key]; exists {
				if _, ok := v.(string); !ok {
					return conflict()
				}
				entry[key] = v
			}
		}
		for _, key := range []string{"unit_price", "discount"} {
			v, exists := line[key]
			if !exists {
				continue
			}
			if v == nil && key == "discount" {
				entry[key] = nil
				continue
			}
			money, ok := v.(map[string]any)
			if !ok {
				return conflict()
			}
			amount, a := money["amount"].(string)
			currency, c := money["currency"].(string)
			if !a || !c {
				return conflict()
			}
			entry[key] = map[string]any{"amount": amount, "currency": currency}
		}
		safeLines = append(safeLines, entry)
	}
	if checkout != "" && typed["receipt_session_id"] != "" && typed["receipt_session_id"] != nil && typed["fiscal_register_id"] != "" && typed["fiscal_register_id"] != nil {
		if !validPreparedTender(typed["total"], public) {
			return conflict()
		}
	}
	typed["lines"] = safeLines
	typed["payments"] = public
	return json.Marshal(typed)
}

var preparedPaymentUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func preparedCents(value any) (int64, bool) {
	money, ok := value.(map[string]any)
	if !ok || money["currency"] != "EUR" {
		return 0, false
	}
	amount, ok := money["amount"].(string)
	if !ok {
		return 0, false
	}
	parts := strings.Split(amount, ".")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) != 2 {
		return 0, false
	}
	digits := parts[0] + parts[1]
	for _, v := range digits {
		if v < '0' || v > '9' {
			return 0, false
		}
	}
	cents, e := strconv.ParseInt(digits, 10, 64)
	return cents, e == nil && cents > 0
}

// Fully prepared association must have the same positive canonical CASH/CARD
// tender plan that FreezeExecutionOrder requires before trusted Attach.
func validPreparedTender(total any, payments []any) bool {
	if len(payments) == 0 || len(payments) > 10 {
		return false
	}
	want, ok := preparedCents(total)
	if !ok {
		return false
	}
	seen := map[string]bool{}
	var sum int64
	const max = int64(^uint64(0) >> 1)
	for _, raw := range payments {
		payment := raw.(map[string]any)
		id, _ := payment["payment_id"].(string)
		kind, _ := payment["type"].(string)
		policy, _ := payment["terminal_policy"].(string)
		cents, ok := preparedCents(payment["amount"])
		if !preparedPaymentUUID.MatchString(id) || seen[id] || !ok || cents > max-sum {
			return false
		}
		if kind == "CARD" {
			if policy != "REQUIRED" && policy != "AUTO_IF_AVAILABLE" {
				return false
			}
		} else if kind == "CASH" {
			if policy != "" && policy != "NONE" {
				return false
			}
		} else {
			return false
		}
		seen[id] = true
		sum += cents
	}
	return sum == want
}
