package main

import (
	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

type RequestDispatcher struct {
	tunio       *TunIO
	connmgr     *WebSocketConnMgr
	addresspool *AdressPool
}

func NewRequestDispatcher(tunio *TunIO, connmgr *WebSocketConnMgr, addresspool *AdressPool) *RequestDispatcher {

	return &RequestDispatcher{tunio: tunio, connmgr: connmgr, addresspool: addresspool}
}

func (r *RequestDispatcher) Dispatch(pkt []byte, conn *WebSocketConn) {

	elog.Info("pkt:", pkt)
	ppkt := PolePacket(pkt)
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

func (r *RequestDispatcher) NewConnection(conn *WebSocketConn, ip string) {
	if ip != "" {
		oldconn := r.connmgr.GetWebSocketConn(ip)
		if oldconn != nil {
			elog.Info("from %v,ip:%v reconnect ok")
			oldconn.Close()
			r.connmgr.AttachIPAddress(ip, conn)
		}
	}
}

func (r *RequestDispatcher) handleAllocIPAddress(pkt PolePacket, conn *WebSocketConn) {
	av := anyvalue.New()

	ip := r.addresspool.Alloc()

	if ip == "" {
		elog.Error("ip alloc fail,no more ip address")
	}
	elog.Infof("alloc ip %v to %v", ip, conn.String())
	av.SetValue("ip", ip)
	body, _ := av.MarshalJSON()
	buf := make([]byte, POLE_PACKET_HEADER_LEN+len(body))
	copy(buf[POLE_PACKET_HEADER_LEN:], body)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(pkt.Cmd())
	resppkt.SetSeq(pkt.Seq())
	conn.Send(resppkt)
	if ip != "" {
		r.connmgr.AttachIPAddress(ip, conn)
	}
}

func (r *RequestDispatcher) handleC2SIPData(pkt PolePacket, conn *WebSocketConn) {
	err := r.tunio.Enqueue(pkt[POLE_PACKET_HEADER_LEN:])
	if err != nil {
		elog.Error("tunio enqueue fail", err)
	}
}

func (r *RequestDispatcher) handleHeartBeat(pkt PolePacket, conn *WebSocketConn) {
	buf := make([]byte, POLE_PACKET_HEADER_LEN)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(CMD_HEART_BEAT)
	resppkt.SetSeq(pkt.Seq())
	conn.Send(resppkt)
}

func (r *RequestDispatcher) handleClientClosed(pkt PolePacket, conn *WebSocketConn) {
	//r.connmgr.DetachIPAddress("", conn)
}
