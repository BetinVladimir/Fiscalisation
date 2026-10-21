package api

import (
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoveryPermissionActualHTTPToTypeScriptConfirmation(t *testing.T) {
	if os.Getenv("MINIPOS_RECOVERY_NODE_INTEROP") != "1" {
		t.Skip("explicit Node+frontend interoperability profile: MINIPOS_RECOVERY_NODE_INTEROP=1")
	}
	h, token, calls := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) { op.RegisterID = "root-register" }, nil, func(o *domain.Order) {
		o.State = "FISCAL_PENDING"
		o.Version = 4
		o.FiscalRegisterID = "root-register"
		o.Total = domain.Money{Amount: "2.50", Currency: "EUR"}
		o.Payments = []domain.OrderPayment{{PaymentID: "payment", Type: "CASH", Amount: o.Total, TerminalPolicy: "NONE"}}
		o.Lines = []domain.Line{}
		o.ShiftID = "00000000-0000-4000-8000-000000000024"
		o.CreatedAt = time.Now().UTC()
		o.UpdatedAt = o.CreatedAt
	})
	server := httptest.NewServer(h)
	defer server.Close()
	source, err := filepath.Abs("../../../BeeMiniPOS/src/checkoutRecovery.ts")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';const{recoverCheckoutReadOnly}=await import(pathToFileURL(process.env.RECOVERY_MODULE));const events=[];const order='00000000-0000-4000-8000-000000000021';const plan={client_operation_id:'pre-prepare-local',receipt_session_id:'pre-prepare-receipt',payments:[{payment_id:'payment',type:'CASH',amount:{amount:'2.50',currency:'EUR'},terminal_policy:'NONE'}],expected_total:{amount:'2.50',currency:'EUR'}};const durable={sale_id:order,source_order_id:order,sale_version:2,created_at:'2026-10-21T00:00:00Z',phase:'AUTHORIZING',plan};const frozen=JSON.stringify(durable);const result=await recoverCheckoutReadOnly({durable,scope:{shiftID:'00000000-0000-4000-8000-000000000024',registerID:'register',operatorCode:'operator',fiscalRegisterID:'root-register'},live:()=>true,read:async path=>{events.push('GET '+path);const r=await fetch(process.env.RECOVERY_URL+'/public/v1/minipos'+path,{headers:{Authorization:'Bearer '+process.env.RECOVERY_TOKEN,'X-App-Instance-Id':'instance','X-Api-Version':'2026-08-07'}});assert.equal(r.status,200,await r.clone().text());return r.json()},journal:{async remove(id){events.push('REMOVE '+id)}}});assert.equal(result.kind,'FISCALIZED');assert.equal(JSON.stringify(durable),frozen);assert.deepEqual(events,['GET /orders/'+order,'GET /orders/'+order+'/recovery','GET /orders/'+order+'/operations/00000000-0000-4000-8000-000000000022','GET /orders/'+order,'REMOVE '+order]);console.log(JSON.stringify({status:'PASS_ACTUAL_HTTP_READ_ONLY_RECOVERY',events,financialWrites:0}));`
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for actual recovery contract interop", err)
	}
	command := exec.Command(node, "--experimental-strip-types", "--input-type=module", "-e", script)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "RECOVERY_URL=" + server.URL, "RECOVERY_TOKEN=" + token, "RECOVERY_MODULE=" + source}
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("TypeScript recovery failed: %v\n%s", err, out)
	}
	server.Close()
	if *calls != 1 {
		t.Fatalf("expected one root operation GET, got%d", *calls)
	}
	t.Log(string(out))
}
