package main
import("crypto/tls";"crypto/x509";"fmt";"io";"net";"os";"os/exec";"time")
type pipes struct{io.ReadCloser; w io.WriteCloser}
func(p pipes)Write(b []byte)(int,error){return p.w.Write(b)}
func(p pipes)Close()error{p.w.Close();return p.ReadCloser.Close()}
func(p pipes)LocalAddr()net.Addr{return &net.UnixAddr{Name:"Go independent TLS",Net:"pipe"}}
func(p pipes)RemoteAddr()net.Addr{return p.LocalAddr()}
func(p pipes)SetDeadline(time.Time)error{return nil};func(p pipes)SetReadDeadline(time.Time)error{return nil};func(p pipes)SetWriteDeadline(time.Time)error{return nil}
func main(){cmd:=exec.Command("./c-fixture","server");cmd.Stderr=os.Stderr;r,_:=cmd.StdoutPipe();w,_:=cmd.StdinPipe();if e:=cmd.Start();e!=nil{panic(e)};defer cmd.Process.Kill();deadline:=time.AfterFunc(10*time.Second,func(){_ = cmd.Process.Kill()});defer deadline.Stop();pem,_:=os.ReadFile("ca.pem");ca:=x509.NewCertPool();ca.AppendCertsFromPEM(pem);c:=tls.Client(pipes{r,w},&tls.Config{RootCAs:ca,ServerName:"adapter.spike.test",MinVersion:tls.VersionTLS13,MaxVersion:tls.VersionTLS13});if e:=c.Handshake();e!=nil{panic(e)};msg:=[]byte("independent Go client synthetic read-only message");if _,e:=c.Write(msg);e!=nil{panic(e)};got:=make([]byte,len(msg));if _,e:=io.ReadFull(c,got);e!=nil{panic(e)};if string(got)!=string(msg){panic("payload")};if e:=cmd.Wait();e!=nil{panic(e)};fmt.Printf("PASS independent Go ↔ mbedTLS memory BIO TLS1.3 suite=%x\n",c.ConnectionState().CipherSuite)}
