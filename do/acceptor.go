package do

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
)

type upstreamProvider = func() (net.Conn, error)

type acceptor struct {
	ctx      context.Context
	logger   hclog.Logger
	listener net.Listener
	provider upstreamProvider
	settings *acceptorSettings

	chanFinished chan struct{}
}

type acceptorSettings struct {
	min int
	max int
}

func newAcceptor(ctx context.Context, settings *acceptorSettings, logger hclog.Logger, listener net.Listener, provider upstreamProvider) *acceptor {
	return &acceptor{
		ctx:          ctx,
		logger:       logger,
		listener:     listener,
		provider:     provider,
		settings:     settings,
		chanFinished: make(chan struct{}),
	}
}

func (a *acceptor) start() <-chan struct{} {
	go a.acceptOnce()
	return a.chanFinished
}

func (a *acceptor) acceptOnce() {
	select {
	case <-a.ctx.Done():
		a.logger.Debug("finished")
		close(a.chanFinished)
		return
	default:

		a.logger.Debug("waiting for connection")

		if l, ok := a.listener.(*net.TCPListener); ok {
			// give it some random time so we always have an acceptor
			// TODO: extract to top level variable
			n := time.Duration(rand.Intn(a.settings.max-a.settings.min) + a.settings.min)
			l.SetDeadline(time.Now().Add(n * time.Millisecond))
		}

		// Connection setup:
		downstream, err := a.listener.Accept()
		if err != nil {
			// https://github.com/golang/go/blob/7846e25418a087ca15122b88fc179405e26bf768/src/net/timeout_test.go#L1158
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				a.logger.Error("failed accepting a connection", "reason", err)
			}
			go a.acceptOnce()
			return
		}

		upstream, err := a.provider()
		if err != nil {
			a.logger.Error("failed setting up proxy to upstream", "reason", err)
			downstream.Close()
			go a.acceptOnce()
			return
		}

		newHandler(a.ctx, downstream, upstream).run()
		go a.acceptOnce()
		return

	}
}
