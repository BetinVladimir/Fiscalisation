package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"
)

type pipes struct {
	io.ReadCloser
	w io.WriteCloser
}

func (p pipes) Write(b []byte) (int, error)      { return p.w.Write(b) }
func (p pipes) Close() error                     { p.w.Close(); return p.ReadCloser.Close() }
func (p pipes) LocalAddr() net.Addr              { return &net.UnixAddr{Name: "independent Go server", Net: "pipe"} }
func (p pipes) RemoteAddr() net.Addr             { return p.LocalAddr() }
func (p pipes) SetDeadline(time.Time) error      { return nil }
func (p pipes) SetReadDeadline(time.Time) error  { return nil }
func (p pipes) SetWriteDeadline(time.Time) error { return nil }
func main() {
	for _, mode := range []string{"positive", "client-only"} {
		leaf := "server.pem"
		if mode == "client-only" {
			leaf = "client-only.pem"
		}
		cert, e := tls.LoadX509KeyPair(leaf, "server-key.pem")
		if e != nil {
			panic(e)
		}
		cmd := exec.Command("./c-peer-client", "ca.pem", "adapter.spike.test", mode)
		r, _ := cmd.StdoutPipe()
		w, _ := cmd.StdinPipe()
		var log bytes.Buffer
		cmd.Stderr = &log
		if e = cmd.Start(); e != nil {
			panic(e)
		}
		watchdog := time.AfterFunc(10*time.Second, func() { cmd.Process.Kill() })
		c := tls.Server(pipes{r, w}, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &cert, nil }})
		handshake := c.Handshake()
		if mode == "positive" {
			if handshake != nil {
				panic(handshake)
			}
			message := make([]byte, len("actual C client independent Go server"))
			if _, e = io.ReadFull(c, message); e != nil || string(message) != "actual C client independent Go server" {
				panic(fmt.Sprint("message", e))
			}
		} else if handshake == nil {
			panic("forbidden EKU handshake accepted")
		}
		w.Close()
		e = cmd.Wait()
		watchdog.Stop()
		if e != nil {
			panic(fmt.Sprintf("C client failure %v %s", e, log.String()))
		}
		if mode == "client-only" && !bytes.Contains(log.Bytes(), []byte("PASS actual C client rejects")) {
			panic("rejection evidence absent")
		}
		fmt.Printf("PASS independent Go server → mbedTLS client mode=%s\n%s", mode, log.String())
	}
}
