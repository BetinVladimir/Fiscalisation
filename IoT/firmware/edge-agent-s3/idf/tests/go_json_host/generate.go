package main
import("encoding/json";"crypto/sha256";"fiscalisation/fiscal-backend/internal/mqttclient";"fmt";"math";"math/rand";"fiscalisation/fiscal-backend/internal/domain")
func main(){
 vals:=[]float64{0,math.Copysign(0,-1),1e-6,1e-7,1e20,1e21,math.SmallestNonzeroFloat64,math.MaxFloat64,math.Nextafter(1e-6,0),math.Nextafter(1e21,0),333333333.3333333,1000000000000000100}
 r:=rand.New(rand.NewSource(22));for i:=0;i<100000;i++{vals=append(vals,math.Float64frombits(r.Uint64()))};for _,v:=range vals{if math.IsNaN(v)||math.IsInf(v,0){continue};b,_:=json.Marshal(v);fmt.Printf("F\t%016x\t%s\n",math.Float64bits(v),b)}
 inputs:=[]string{`{"z":1,"a":{"z":0.000001,"a":["<>&\u2028\u2029",-0,1e21]},"command":{"z":true,"a":null}}`,`{"operation_sequence":5,"unp_sequence":3,"command":{"amount":12.30,"name":"Кафе & <sale>"}}`,`{"\\":1,"ä":2,"a":3,"😀":4}`}
 for _,s:=range inputs{var v any;if err:=json.Unmarshal([]byte(s),&v);err!=nil{panic(err)};b,_:=json.Marshal(v);fmt.Printf("M\t%s\t%s\n",s,b)}
 var payload map[string]any;json.Unmarshal([]byte(inputs[1]),&payload)
 e:=domain.DeviceEventEnvelope{EventID:"e<>&",OperationID:"op",DeviceID:"edge",JournalSeq:9223372036854775807,EventType:"ACCEPTED",OccurredAt:"2026-10-17T00:00:00Z",Payload:payload}
 b,_:=json.Marshal(e);fmt.Printf("E\t%s\t%s\n",b,domain.DeviceEventHash(e))
 e.EventHash=domain.DeviceEventHash(e);signature:="kid:sig";e.Signature=&signature
 batch:=domain.EdgeSyncBatch{EdgeID:"edge",SchemaVersion:"2026-08-07",FirstSeq:e.JournalSeq,LastSeq:e.JournalSeq,Events:[]domain.DeviceEventEnvelope{e}}
 b,_=json.Marshal(batch);fmt.Printf("B\t%s\t%s\n",b,domain.EdgeBatchHash(batch))
 sale:=domain.Sale{ID:"sale-1",TenantID:"tenant-1",RegisterID:"reg-1",OperatorID:"A001",Lines:[]domain.SaleLine{{LineID:"line-1",Name:"Кафе & <чай>",Quantity:"1.000",UnitPrice:domain.Money{Amount:"2.65",Currency:"EUR"},TaxGroup:"B"}},FiscalDevice:domain.FiscalDeviceSnapshot{DeviceID:"device-1",BindingVersion:7}}
 command,err:=mqttclient.NewBridge("0123456789abcdef0123456789abcdef").Prepare(domain.Operation{ID:"op-1",Type:"FISCAL_SALE"},sale,domain.PaymentRequest{})
 if err!=nil{panic(err)};body:=command.Data["body"].(string);var envelope map[string]any;json.Unmarshal([]byte(body),&envelope);pb,_:=json.Marshal(envelope["payload"]);fmt.Printf("P\t%s\t%s\n",pb,envelope["payload_sha256"]);sum:=sha256.Sum256([]byte(body));fmt.Printf("C\t%s\t%x\n",body,sum)

}
