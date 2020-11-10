package main

import (
	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

type RequestDispatcher struct {
	tunio *TunIO
}

func NewRequestDispatcher(tunio *TunIO) *RequestDispatcher {

	return &RequestDispatcher{tunio: tunio}
}

func (r *RequestDispatcher) Dispatch(pkt []byte, conn *WebSocketConn) {

	ppkt := PolePacket(pkt)
	ppkt.Cmd()

	switch ppkt.Cmd() {
	case CMD_ALLOC_IPADDR:
		r.handleAllocIPAddress(ppkt, conn)
	case CMD_C2S_IPDATA:
		r.handleC2SIPData(ppkt, conn)
	case CMD_HEART_BEAT:
		r.handleHeartBeat(ppkt, conn)
	case CMD_CLIENT_CLOSED:
		r.handleClientClosed(ppkt, conn)
	default:
		elog.Error("invalid pkt cmd=", ppkt.Cmd())
	}
}

func (r *RequestDispatcher) handleAllocIPAddress(pkt PolePacket, conn *WebSocketConn) {
	av := anyvalue.New()
	ip := "10.8.0.7"
	av.SetValue("ip", ip)
	body, _ := av.MarshalJSON()
	buf := make([]byte, POLE_PACKET_HEADER_LEN+len(body))
	copy(buf[POLE_PACKET_HEADER_LEN:], body)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(pkt.Cmd())
	resppkt.SetSeq(pkt.Seq())
	conn.Send(resppkt)
	r.tunio.handler.connmgr.AttachIPAddress(ip, conn)
}

func (r *RequestDispatcher) handleC2SIPData(pkt PolePacket, conn *WebSocketConn) {
	err := r.tunio.Enqueue(pkt[POLE_PACKET_HEADER_LEN:])
	if err != nil {
		elog.Error("tunio enqueue fail", err)
	}
}

func (r *RequestDispatcher) handleHeartBeat(pkt PolePacket, conn *WebSocketConn) {

}

func (r *RequestDispatcher) handleClientClosed(pkt PolePacket, conn *WebSocketConn) {
	r.tunio.handler.connmgr.DetachIPAddress("", conn)
}
