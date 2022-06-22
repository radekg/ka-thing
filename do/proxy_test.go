package do

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/radekg/ka-thing/config"
	"github.com/stretchr/testify/assert"
)

func TestSimpleRoundtripProxy(t *testing.T) {

	echoServer := &echo{}
	addr := echoServer.start(t)
	defer echoServer.close()

	proxyCfg := newTestProxyConfig(t, addr)

	proxyCtx, cancelFunc := context.WithCancel(context.Background())
	p := NewProxy(proxyCtx, proxyCfg.WithDefaults(), defaultLogger(t))
	p.Start()

	select {
	case <-p.ReadyNotify():
	case <-p.FailedNotify():
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but got error", p.StartFailureReason())
	case <-time.After(time.Second * 10):
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but it timed out")
	}

	echoServer.echo(t)

	expectedInput := "hello, world"

	conn, err := net.Dial("tcp", p.BoundAddress().String())
	assert.Nil(t, err)

	{
		nwritten, writeErr := connectionWrite(t, conn, []byte(expectedInput))
		assert.Nil(t, writeErr)
		buf := make([]byte, 1024)
		nread, readErr := connectionRead(t, conn, buf)
		assert.Nil(t, readErr)
		assert.Equal(t, nwritten, nread)
		output := string(buf[0:nread])
		assert.Equal(t, expectedInput, output)
	}

	// cancel the context to stop the proxy:
	cancelFunc()
	// stopped:
	<-p.StoppedNotify()
}

func TestProxyHandlesNoTarget(t *testing.T) {

	echoServer := &echo{}
	addr := echoServer.start(t)
	// close immediately so we have an unresponsive target:
	echoServer.close()

	proxyCfg := newTestProxyConfig(t, addr)

	proxyCtx, cancelFunc := context.WithCancel(context.Background())
	p := NewProxy(proxyCtx, proxyCfg.WithDefaults(), defaultLogger(t))
	p.Start()

	select {
	case <-p.ReadyNotify():
	case <-p.FailedNotify():
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but got error", p.StartFailureReason())
	case <-time.After(time.Second * 10):
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but it timed out")
	}

	conn, err := net.Dial("tcp", p.BoundAddress().String())
	assert.Nil(t, err)

	{
		expectedInput := "hello, world"
		nwritten, writeErr := connectionWrite(t, conn, []byte(expectedInput))
		assert.Nil(t, writeErr)
		assert.Equal(t, len(expectedInput), nwritten)
		buf := make([]byte, 1024)
		nread, readErr := connectionRead(t, conn, buf)
		assert.NotNil(t, readErr)
		assert.Equal(t, 0, nread)
	}

	// cancel the context to stop the proxy:
	cancelFunc()
	// stopped:
	<-p.StoppedNotify()
}

func TestProxyHandlesTargetLossDuringCommunication(t *testing.T) {

	echoServer := &echo{}
	addr := echoServer.start(t)

	proxyCfg := newTestProxyConfig(t, addr)

	proxyCtx, cancelFunc := context.WithCancel(context.Background())
	p := NewProxy(proxyCtx, proxyCfg.WithDefaults(), defaultLoggerWithLevel(t, hclog.Debug))
	p.Start()

	select {
	case <-p.ReadyNotify():
	case <-p.FailedNotify():
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but got error", p.StartFailureReason())
	case <-time.After(time.Second * 10):
		assert.NotNil(t, p.StartFailureReason())
		cancelFunc()
		assert.FailNow(t, "expected proxy to start but it timed out")
	}

	echoServer.echo(t)

	// connect test client:
	conn, err := net.Dial("tcp", p.BoundAddress().String())
	assert.Nil(t, err)

	{
		expectedInput := "message 1"
		// write:
		nwritten, writeErr := connectionWrite(t, conn, []byte(expectedInput))
		assert.Nil(t, writeErr)
		// read:
		buf := make([]byte, 1024)
		nread, readErr := connectionRead(t, conn, buf)
		assert.Nil(t, readErr)
		assert.Equal(t, nwritten, nread)
		output := string(buf[0:nread])
		assert.Equal(t, expectedInput, output)
	}

	// close the target:
	echoServer.close()

	{
		expectedInput := "message 2"
		_, writeErr := connectionWrite(t, conn, []byte(expectedInput))
		assert.Nil(t, writeErr)
		buf := make([]byte, 1024)
		nread, readErr := connectionRead(t, conn, buf)
		assert.NotNil(t, readErr)
		assert.Equal(t, 0, nread)
		output := string(buf[0:nread])
		assert.Equal(t, "", output)
	}

	// TODO: this can be eliminated by the proxy waiting for all acceptors to stop gracefully
	//       and those have to wait for all handlers to exit gracefully, too
	<-time.After(time.Millisecond * 200)

	// cancel the context to stop the proxy:
	cancelFunc()
	// stopped:
	<-p.StoppedNotify()
}

type echo struct {
	closed   bool
	listener net.Listener
}

func (e *echo) close() {
	e.closed = true
	e.listener.Close()
}

func (e *echo) start(t *testing.T) net.Addr {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	e.listener = l
	return e.listener.Addr()
}

func (e *echo) echo(t *testing.T) {
	go func() {
		conn, err := e.listener.Accept()
		assert.Nil(t, err)
		for {
			if e.closed {
				return
			}
			buf := make([]byte, 1024)
			nread, err := conn.Read(buf)
			assert.Nil(t, err)
			<-time.After(time.Millisecond * 50) // simulate network latency
			nwritten, err := conn.Write(buf[0:nread])
			assert.Nil(t, err)
			assert.Equal(t, nread, nwritten)
			<-time.After(time.Millisecond * 50) // simulate network latency
		}
	}()
}

func connectionRead(t *testing.T, conn net.Conn, buf []byte) (int, error) {
	conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
	return conn.Read(buf)
}

func connectionWrite(t *testing.T, conn net.Conn, buf []byte) (int, error) {
	conn.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
	return conn.Write(buf)
}

func newTestProxyConfig(t *testing.T, addr net.Addr) *config.ProxyConfig {
	proxyCfg := config.NewProxyConfig()
	proxyCfg.ProxyTarget = addr.String()
	proxyCfg.BindHostPort = "127.0.0.1:0"
	proxyCfg.AcceptorMax = 100
	proxyCfg.AcceptorMin = 50
	proxyCfg.NumAcceptors = 1
	return proxyCfg
}

func defaultLogger(t *testing.T) hclog.Logger {
	l := hclog.Default()
	l.SetLevel(hclog.Info)
	return l
}

func defaultLoggerWithLevel(t *testing.T, level hclog.Level) hclog.Logger {
	l := hclog.Default()
	l.SetLevel(level)
	return l
}
