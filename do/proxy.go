package do

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/radekg/ka-thing/config"
)

// Proxy represents a proxy instance.
type Proxy interface {
	BoundAddress() net.Addr

	Start()

	FailedNotify() <-chan struct{}
	ReadyNotify() <-chan struct{}
	StoppedNotify() <-chan struct{}

	StartFailureReason() error
}

// NewProxy returns an instance of a proxy.
func NewProxy(ctx context.Context, cfg *config.ProxyConfig, logger hclog.Logger) Proxy {
	s := &defaultProxy{
		ctx:         ctx,
		cfg:         cfg,
		logger:      logger,
		chanFailed:  make(chan struct{}),
		chanReady:   make(chan struct{}),
		chanStopped: make(chan struct{}),
	}

	return s

}

type defaultProxy struct {
	ctx context.Context

	cfg *config.ProxyConfig

	chanReady   chan struct{}
	chanStopped chan struct{}
	chanFailed  chan struct{}

	failedError error

	listener net.Listener
	logger   hclog.Logger
}

func (p *defaultProxy) defaultUpstreamProvider() upstreamProvider {
	return func() (net.Conn, error) {
		// TODO: TLS support
		return net.Dial("tcp", p.cfg.ProxyTarget)
	}
}

func (p *defaultProxy) BoundAddress() net.Addr {
	if p.listener != nil {
		return p.listener.Addr()
	}
	return nil
}

func (p *defaultProxy) Start() {

	// TODO: TLS support
	listener, err := net.Listen("tcp", p.cfg.BindHostPort)
	if err != nil {
		p.logger.Error("failed creating TCP listener", "reason", err)
		p.failedError = err
		close(p.chanFailed)
		return
	}

	p.listener = listener

	acceptorConfig := &acceptorSettings{
		min: p.cfg.AcceptorMin,
		max: p.cfg.AcceptorMax,
	}

	go func() {
		waitGroup := &sync.WaitGroup{}
		for i := 1; i <= p.cfg.NumAcceptors; i = i + 1 {
			waitGroup.Add(1)
			go func(id int) {
				<-newAcceptor(p.ctx, acceptorConfig, p.logger.Named(fmt.Sprintf("acceptor-%d", id)), p.listener, p.defaultUpstreamProvider()).start()
				waitGroup.Done()
			}(i)
		}
		waitGroup.Wait()
		close(p.chanStopped)
		p.logger.Info("server finished")
	}()

	close(p.chanReady)

}

// ReadyNotify returns a channel that will be closed when the server is ready to serve client requests.
func (p *defaultProxy) ReadyNotify() <-chan struct{} {
	return p.chanReady
}

// FailedNotify returns a channel that will be closed when the server has failed to start.
func (p *defaultProxy) FailedNotify() <-chan struct{} {
	return p.chanFailed
}

// StoppedNotify returns a channel that will be closed when the server has stopped.
func (p *defaultProxy) StoppedNotify() <-chan struct{} {
	return p.chanStopped
}

// StartFailureReason returns an error which caused the server not to start.
func (p *defaultProxy) StartFailureReason() error {
	return p.failedError
}
