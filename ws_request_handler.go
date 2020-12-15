package main

import (
	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

type WSRequestHandler struct {
	tunio   *TunIO
	connmgr *ConnMgr
}

func NewWSRequestHandler() *WSRequestHandler {

	return &WSRequestHandler{}
}

func (r *WSRequestHandler) SetTunIO(tunio *TunIO) {
	r.tunio = tunio
}

func (r *WSRequestHandler) SetConnMgr(connmgr *ConnMgr) {
	r.connmgr = connmgr
}

func (r *WSRequestHandler) OnRequest(pkt []byte, conn *WebSocketConn) {

	ppkt := PolePacket(pkt)
	switch ppkt.Cmd() {
	case CMD_ALLOC_IPADDR:
		elog.Info("received alloc ip adress request from", conn.String())
		r.handleAllocIPAddress(ppkt, conn)
	case CMD_C2S_IPDATA:
		r.handleC2SIPData(ppkt, conn)
	case CMD_HEART_BEAT:
		//elog.Info("received heart beat request", conn.String())
		r.handleHeartBeat(ppkt, conn)
	case CMD_CLIENT_CLOSED:
		r.handleClientClose(ppkt, conn)
	default:
		elog.Error("invalid pkt cmd=", ppkt.Cmd())
	}
}

func (r *WSRequestHandler) OnConnection(conn *WebSocketConn, user string, ip string) {
	if ip != "" {
		oldconn := r.connmgr.GetConnByIP(ip)
		if oldconn != nil {
			oldconn.Close(true)
		}
		r.connmgr.AttachIPAddressToConn(ip, conn)
		elog.Infof("from %v,ip:%v reconnect ok", conn.String(), ip)

	}
	r.connmgr.AttachUserToConn(user, conn)

}

func (r *WSRequestHandler) handleAllocIPAddress(pkt PolePacket, conn *WebSocketConn) {
	av := anyvalue.New()

	ip := r.connmgr.AllocAddress()

	if ip == "" {
		elog.Error("ip alloc fail,no more ip address")
	}
	elog.Infof("alloc ip %v to %v", ip, conn.String())
	av.Set("ip", ip)
	av.Set("dns", Config.Get("dns_server").AsStr())
	body, _ := av.MarshalJSON()
	buf := make([]byte, POLE_PACKET_HEADER_LEN+len(body))
	copy(buf[POLE_PACKET_HEADER_LEN:], body)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(pkt.Cmd())
	resppkt.SetSeq(pkt.Seq())
	resppkt.SetLen(uint16(len(buf)))
	conn.Send(resppkt)
	if ip != "" {
		r.connmgr.AttachIPAddressToConn(ip, conn)
		r.connmgr.AttachUserToIP(r.connmgr.GetConnAttachUser(conn), ip)
	}
}

func (r *WSRequestHandler) handleC2SIPData(pkt PolePacket, conn *WebSocketConn) {

	if r.tunio != nil {
		err := r.tunio.Enqueue(pkt[POLE_PACKET_HEADER_LEN:])
		if err != nil {
			elog.Error("tunio enqueue fail", err)
		}
	}
}

func (r *WSRequestHandler) handleHeartBeat(pkt PolePacket, conn *WebSocketConn) {
	buf := make([]byte, POLE_PACKET_HEADER_LEN)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(CMD_HEART_BEAT)
	resppkt.SetSeq(pkt.Seq())
	resppkt.SetLen(uint16(len(buf)))
	conn.Send(resppkt)
	r.connmgr.UpdateConnActiveTime(conn)
}

func (r *WSRequestHandler) handleClientClose(pkt PolePacket, conn *WebSocketConn) {
	elog.Info(conn.String(), "proactive close")
	r.connmgr.RelelaseAddress(r.connmgr.GeIPByConn(conn))
}

func (r *WSRequestHandler) OnClosed(conn *WebSocketConn, proactive bool) {

	elog.Info("connection closed event from", conn.String())

	r.connmgr.DetachIPAddressFromConn(conn)
	r.connmgr.DetachUserFromConn(conn)

	//just process proactive close event
	if proactive {
		elog.Info(conn.String(), "proactive close")
		r.connmgr.RelelaseAddress(r.connmgr.GeIPByConn(conn))
	}

}
