package do

import (
	"context"
	"net"
)

func newHandler(ctx context.Context, downstream net.Conn, upstream net.Conn) *handler {
	return &handler{
		ctx:         ctx,
		downstream:  downstream,
		upstream:    upstream,
		chanIngress: make(chan []byte),
		chanEgress:  make(chan []byte),
	}
}

type handler struct {
	ctx         context.Context
	downstream  net.Conn
	upstream    net.Conn
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
		buf := make([]byte, 1024*1024)
		nread, err := h.downstream.Read(buf)
		if err != nil {
			// TODO: handle error
		}
		h.chanIngress <- buf[0:nread]
		go h.readDownstream()
	}
}

func (h *handler) readUpstream() {
	select {
	case <-h.ctx.Done():
		return
	default:
		buf := make([]byte, 1024*1024)
		nread, err := h.upstream.Read(buf)
		if err != nil {
			// TODO: handle error
		}
		h.chanEgress <- buf[0:nread]
		go h.readUpstream()
	}
}

func (h *handler) writeDownstream() {
	select {
	case <-h.ctx.Done():
		return
	case data := <-h.chanEgress:
		nwritten, err := h.downstream.Write(data)
		if err != nil {
			// TODO: handle error
		}
		if nwritten > 0 {
			// TODO: do something with this number
		}
		go h.writeDownstream()
	}
}

func (h *handler) writeUpstream() {
	select {
	case <-h.ctx.Done():
		return
	case data := <-h.chanIngress:
		nwritten, err := h.upstream.Write(data)
		if err != nil {
			// TODO: handle error
		}
		if nwritten > 0 {
			// TODO: do something with this number
		}
		go h.writeUpstream()
	}
}
