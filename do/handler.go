package do

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

func newHandler(ctx context.Context, logger hclog.Logger, downstream net.Conn, upstream net.Conn) *handler {
	handlerCtx, handlerCancelFunc := context.WithCancel(ctx)
	return &handler{
		ctx:         handlerCtx,
		ctxCancel:   handlerCancelFunc,
		downstream:  downstream,
		upstream:    upstream,
		logger:      logger,
		chanIngress: make(chan []byte),
		chanEgress:  make(chan []byte),
		closeMutex:  &sync.Mutex{},
	}
}

type handler struct {
	ctx         context.Context
	ctxCancel   context.CancelFunc
	downstream  net.Conn
	upstream    net.Conn
	logger      hclog.Logger
	chanIngress chan []byte
	chanEgress  chan []byte

	closeMutex *sync.Mutex
	closed     bool
}

func (h *handler) run() {
	go h.readDownstream()
	go h.writeDownstream()
	go h.readUpstream()
	go h.writeUpstream()
}

// do not call close directly:
func (h *handler) close(reason string) {
	h.closeMutex.Lock()
	if !h.closed {
		h.closed = true
		h.logger.Info("closing handler", "upstream", h.upstream.RemoteAddr().String(), "reason", reason)
		h.closeMutex.Unlock()
		h.ctxCancel()
		h.downstream.Close()
		h.upstream.Close()
		return
	}
	h.closeMutex.Unlock()
}

func (h *handler) readDownstream() {
	select {
	case <-h.ctx.Done():
		h.logger.Debug("stopping readDownstream")
	default:
		buf := make([]byte, 1024*1024)                                       // TODO: have only one instance of this buffer
		h.downstream.SetReadDeadline(time.Now().Add(time.Millisecond * 100)) // TODO: this needs to be configurable
		nread, err := h.downstream.Read(buf)
		if err != nil {
			// TODO: handle broken pipe gracefully
			if err == io.EOF {
				h.close("downstream lost")
			} else if errors.Is(err, os.ErrDeadlineExceeded) {
				// it's okay, let it go!
			} else if errors.Is(err, net.ErrClosed) {
				// it's okay, let it go!
			} else {
				h.logger.Error("failed reading downstream", "reason", err)
			}
		} else {
			if nread > 0 {
				h.chanIngress <- buf[0:nread]
			}
		}
		go h.readDownstream()
	}
}

func (h *handler) readUpstream() {
	select {
	case <-h.ctx.Done():
		h.logger.Debug("stopping readUpstream")
	default:
		buf := make([]byte, 1024*1024)                                     // TODO: have only one instance of this buffer
		h.upstream.SetReadDeadline(time.Now().Add(time.Millisecond * 100)) // TODO: this needs to be configurable
		nread, err := h.upstream.Read(buf)
		if err != nil {
			// TODO: handle broken pipe gracefully
			if err == io.EOF {
				h.close("upstream lost")
			} else if errors.Is(err, os.ErrDeadlineExceeded) {
				// it's okay, let it go!
			} else if errors.Is(err, net.ErrClosed) {
				// it's okay, let it go!
			} else {
				h.logger.Error("failed reading upstream", "reason", err)
			}
		} else {
			if nread > 0 {
				h.chanEgress <- buf[0:nread]
			}
		}
		go h.readUpstream()
	}
}

func (h *handler) writeDownstream() {
	select {
	case <-h.ctx.Done():
		h.logger.Debug("stopping writeDownstream")
	case data := <-h.chanEgress:
		datalen := len(data)
		nwritten, err := h.downstream.Write(data)
		if err != nil {
			// TODO: handle broken pipe gracefully
			h.logger.Error("failed writing downstream", "reason", err)
		}
		if nwritten < datalen {
			// TODO: do something about it
			h.logger.Error("not all data written downstream", "expected", datalen, "written", nwritten)
		}
		go h.writeDownstream()
	}
}

func (h *handler) writeUpstream() {
	select {
	case <-h.ctx.Done():
		h.logger.Debug("stopping writeUpstream")
	case data := <-h.chanIngress:
		datalen := len(data)
		nwritten, err := h.upstream.Write(data)
		if err != nil {
			// TODO: handle broken pipe gracefully
			h.logger.Error("failed writing upstream", "reason", err)
		}
		if nwritten < datalen {
			// TODO: do something about it
			h.logger.Error("not all data written downstream", "expected", datalen, "written", nwritten)
		}
		go h.writeUpstream()
	}
}
