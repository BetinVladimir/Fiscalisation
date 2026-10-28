package main

import (
 "bufio"
 "crypto/hmac"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "fmt"
 "math"
 "math/rand"
 "os"
 "time"
 "fiscalisation/fiscal-backend/internal/domain"
)
func main(){
 out:=bufio.NewWriter(os.Stdout);defer out.Flush()
 row:=func(kind string,input any,v any){b,err:=json.Marshal(v);if err!=nil{panic(err)};r,_:=json.Marshal(map[string]any{"kind":kind,"input":input,"want":string(b)});fmt.Fprintln(out,string(r))}
 for _,s:=range []string{"<>&\u2028\u2029","a\"\\/\b\f\n\r\t\x00\x1f","Кафе € 😀","\ufffd"}{row("string",s,s)}
 row("map",`{"𐀀":"<>&","":"line\u2028","z":{"β":1,"a":[1e-7,-0.0,1e21,0.000001]}}`,map[string]any{"\U00010000":"<>&","\ue000":"line\u2028","z":map[string]any{"β":float64(1),"a":[]any{1e-7,math.Copysign(0,-1),1e21,0.000001}}})
 for _,i:=range []int64{0,1,-1,9007199254740991,9007199254740992,9007199254740993,math.MaxInt64,math.MinInt64}{row("int64",fmt.Sprint(i),i)}
 ds:=[]float64{0,math.Copysign(0,-1),1,1e-7,1e-6,1e20,1e21,1e23,math.SmallestNonzeroFloat64,math.MaxFloat64,math.Nextafter(1e-6,0),math.Nextafter(1e21,0),math.Nextafter(1,0),math.Nextafter(1,2)}
 for _,v:=range ds {row("float64",fmt.Sprintf("%016x",math.Float64bits(v)),v)}
 for exponent:=-1074;exponent<=1023;exponent++ {v:=math.Ldexp(1,exponent);for _,value:=range []float64{v,math.Nextafter(v,0),math.Nextafter(v,math.Inf(1))}{if !math.IsInf(value,0){row("float64",fmt.Sprintf("%016x",math.Float64bits(value)),value)}}}
 rng:=rand.New(rand.NewSource(20261018));for i:=0;i<2500;i++ {bits:=rng.Uint64();v:=math.Float64frombits(bits);if !math.IsNaN(v)&&!math.IsInf(v,0){row("float64",fmt.Sprintf("%016x",bits),v)}}
 fs:=[]float32{0,float32(math.Copysign(0,-1)),1e-7,1e-6,1e20,1e21,math.SmallestNonzeroFloat32,math.MaxFloat32};for _,v:=range fs{row("float32",fmt.Sprintf("%08x",math.Float32bits(v)),v)}
 for exponent:=-149;exponent<=127;exponent++ {v:=float32(math.Ldexp(1,exponent));for _,value:=range []float32{v,math.Nextafter32(v,0),math.Nextafter32(v,float32(math.Inf(1)))}{if !math.IsInf(float64(value),0){row("float32",fmt.Sprintf("%08x",math.Float32bits(value)),value)}}}
 for i:=0;i<1000;i++ {bits:=rng.Uint32();v:=math.Float32frombits(bits);if !math.IsNaN(float64(v))&&!math.IsInf(float64(v),0){row("float32",fmt.Sprintf("%08x",bits),v)}}
 previous,signature:="prev<>&","key:<signed>&"
 event:=domain.DeviceEventEnvelope{EventID:"event<>&",OperationID:"op\u2028",DeviceID:"edge😀",JournalSeq:math.MaxInt64,EventType:"RECEIPT<RESULT>",OccurredAt:"2026-10-18T01:02:03.123456789Z",Payload:map[string]any{"\ue000":"<>&","\U00010000":1e23},PrevHash:&previous,EventHash:"hash<>&",Signature:&signature}
 for _,nilValues:=range []bool{false,true}{if nilValues{event.PrevHash=nil;event.Signature=nil;event.EventHash=""};b,_:=json.Marshal(event);row("event",string(b),event)
 batch:=domain.EdgeSyncBatch{EdgeID:"edge😀",SchemaVersion:"2026-08-07",FirstSeq:math.MaxInt64-1,LastSeq:math.MaxInt64,PreviousAcknowledgedHash:event.PrevHash,Events:[]domain.DeviceEventEnvelope{event},BatchSHA256:"hash<>&",Signature:signature};b,_=json.Marshal(batch);row("batch",string(b),batch)}
 for _,nilArrays:=range []bool{false,true}{
  ack:=domain.SyncAck{AckID:"ack<>&",EdgeID:"edge😀",CommittedThroughSeq:math.MaxInt64,CommittedEventHash:"<hash>",CommittedAt:time.Date(2026,10,18,1,2,3,123456789,time.UTC)}
  if !nilArrays {ack.OperationResults=[]domain.SyncOperationResult{{OperationID:"op\u2028",State:"<ok>&",Version:math.MaxInt64}};ack.Rejected=[]map[string]any{{"\ue000":"<>&","\U00010000":[]any{1e23,1e-7,math.Copysign(0,-1)}}}}
  raw,_:=json.Marshal(ack);mac:=hmac.New(sha256.New,[]byte("0123456789abcdef0123456789abcdef"));mac.Write(raw);ack.Signature=base64.RawURLEncoding.EncodeToString(mac.Sum(nil));signed,_:=json.Marshal(ack);row("ack",string(signed),string(raw))
 }
}
