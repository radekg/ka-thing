package do

import (
	"context"
	"net"

	"github.com/hashicorp/go-hclog"
)

func newHandler(ctx context.Context, logger hclog.Logger, downstream net.Conn, upstream net.Conn) *handler {
	return &handler{
		ctx:         ctx,
		downstream:  downstream,
		upstream:    upstream,
		logger:      logger,
		chanIngress: make(chan []byte),
		chanEgress:  make(chan []byte),
	}
}

type handler struct {
	ctx         context.Context
	downstream  net.Conn
	upstream    net.Conn
	logger      hclog.Logger
	chanIngress chan []byte
	chanEgress  chan []byte
}

func (h *handler) run() {
	go h.readDownstream()
	go h.writeDownstream()
	go h.readUpstream()
	go h.writeUpstream()
}

func (h *handler) readDownstream() {
	select {
	case <-h.ctx.Done():
		return
	default:
		buf := make([]byte, 1024*1024) // TODO: have only one instance of this buffer
		nread, err := h.downstream.Read(buf)
		if err != nil {
			// TODO: handle broken pipe gracefully
			// TODO: handle EOF gracefully
			h.logger.Error("failed reading downstream", "reason", err)
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
		return
	default:
		buf := make([]byte, 1024*1024) // TODO: have only one instance of this buffer
		nread, err := h.upstream.Read(buf)
		if err != nil {
			// TODO: handle broken pipe gracefully
			// TODO: handle EOF gracefully
			h.logger.Error("failed reading upstream", "reason", err)
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
		return
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
		return
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
