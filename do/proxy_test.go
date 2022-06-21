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

	proxyCfg := config.NewProxyConfig()
	proxyCfg.ProxyTarget = addr.String()
	proxyCfg.BindHostPort = "127.0.0.1:0"
	proxyCfg.AcceptorMax = 100
	proxyCfg.AcceptorMin = 50
	proxyCfg.NumAcceptors = 2

	proxyCtx, cancelFunc := context.WithCancel(context.Background())
	p := NewProxy(proxyCtx, proxyCfg.WithDefaults(), hclog.Default())
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
	conn.Write([]byte(expectedInput))
	buf := make([]byte, 1024)
	nread, err := conn.Read(buf)
	assert.Nil(t, err)
	output := string(buf[0:nread])

	assert.Equal(t, expectedInput, output)

	cancelFunc()

	<-p.StoppedNotify()

	t.Log("all done")

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
			nwritten, err := conn.Write(buf[0:nread])
			assert.Nil(t, err)
			assert.Equal(t, nread, nwritten)
		}
	}()
}
