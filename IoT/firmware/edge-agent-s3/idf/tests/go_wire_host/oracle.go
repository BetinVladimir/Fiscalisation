package main
import("os";"fmt";"encoding/json";"encoding/base64";"crypto/hmac";"crypto/sha256";"time";"fiscalisation/fiscal-backend/internal/domain";"fiscalisation/fiscal-backend/internal/mqttclient")
func main(){switch os.Args[1]{case "command":
 sale:=domain.Sale{ID:"sale-1",TenantID:"tenant",RegisterID:"register",OperatorID:"A001",Lines:[]domain.SaleLine{{LineID:"00000000-0000-4000-8000-000000000003",Name:"Кафе & <чай>",Quantity:"1.000",UnitPrice:domain.Money{Amount:"2.65",Currency:"EUR"},TaxGroup:"A"}},Payments:[]domain.PaymentRecord{{PaymentID:"00000000-0000-4000-8000-000000000004",Type:"CASH",Amount:domain.Money{Amount:"2.65",Currency:"EUR"}}},FiscalDevice:domain.FiscalDeviceSnapshot{DeviceID:"edge",BindingVersion:1}}
 op:=domain.Operation{ID:"00000000-0000-4000-8000-000000000001",ReceiptSessionID:"00000000-0000-4000-8000-000000000002",Type:"SALE_FINALIZE"}
 cmd,err:=mqttclient.NewBridge("kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk").Prepare(op,sale,domain.PaymentRequest{});if err!=nil{panic(err)};os.WriteFile(os.Args[2],[]byte(cmd.Data["body"].(string)),0600)
 case "ack":
 raw,err:=os.ReadFile(os.Args[2]);if err!=nil{panic(err)};var b domain.EdgeSyncBatch;if err=json.Unmarshal(raw,&b);err!=nil{panic(err)};if domain.EdgeBatchHash(b)!=b.BatchSHA256{panic("ACTUAL GO BATCH HASH MISMATCH")};for _,e:=range b.Events{if domain.DeviceEventHash(e)!=e.EventHash{panic("ACTUAL GO EVENT HASH MISMATCH")}}
 a:=domain.SyncAck{AckID:"ack<>&",EdgeID:b.EdgeID,CommittedThroughSeq:b.LastSeq,CommittedEventHash:b.Events[len(b.Events)-1].EventHash,CommittedAt:time.Date(2026,10,17,1,2,3,123456789,time.UTC),OperationResults:[]domain.SyncOperationResult{{OperationID:"op",State:"<approved>&",Version:9223372036854775807}},Rejected:[]map[string]any{{"z":1e-7,"a":map[string]any{"z":"<>&\u2028\u2029\u0000","a":1e21}}}}
 unsigned,_:=json.Marshal(a);m:=hmac.New(sha256.New,[]byte("kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"));m.Write(unsigned);a.Signature=base64.RawURLEncoding.EncodeToString(m.Sum(nil));encoded,_:=json.Marshal(a);os.WriteFile(os.Args[3],encoded,0600);fmt.Println("actual Go domain verifies firmware event/batch hashes; signed typed ACK emitted")
 }}
