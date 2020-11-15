package main

import (
	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
)

type RequestDispatcher struct {
	tunio   *TunIO
	connmgr *WebSocketConnMgr
	count   int
}

func NewRequestDispatcher() *RequestDispatcher {

	return &RequestDispatcher{}
}

func (r *RequestDispatcher) SetTunIO(tunio *TunIO) {
	r.tunio = tunio
}

func (r *RequestDispatcher) SetWebSocketConnMgr(connmgr *WebSocketConnMgr) {
	r.connmgr = connmgr
}

func (r *RequestDispatcher) Dispatch(pkt []byte, conn *WebSocketConn) {

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
		elog.Info("connection closed event from", conn.String())
		r.handleClientClosed(ppkt, conn)
	default:
		elog.Error("invalid pkt cmd=", ppkt.Cmd())
	}
}

func (r *RequestDispatcher) NewConnection(conn *WebSocketConn, user string, ip string) {
	if ip != "" {
		oldconn := r.connmgr.GetWebSocketConnByIP(ip)
		if oldconn != nil {
			oldconn.Close(true)
		}
		r.connmgr.AttachIPAddress(ip, conn)
		elog.Infof("from %v,ip:%v reconnect ok", conn.String(), ip)

	}
	r.connmgr.AttachUserToConn(user, conn)

}

func (r *RequestDispatcher) handleAllocIPAddress(pkt PolePacket, conn *WebSocketConn) {
	av := anyvalue.New()

	ip := r.connmgr.AllocAddress()

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
		r.connmgr.AttachUserToIP(r.connmgr.GetConnAttachUser(conn), ip)
	}
}

func (r *RequestDispatcher) handleC2SIPData(pkt PolePacket, conn *WebSocketConn) {

	if r.tunio != nil {
		err := r.tunio.Enqueue(pkt[POLE_PACKET_HEADER_LEN:])
		if err != nil {
			elog.Error("tunio enqueue fail", err)
		}
	}
}

func (r *RequestDispatcher) handleHeartBeat(pkt PolePacket, conn *WebSocketConn) {
	buf := make([]byte, POLE_PACKET_HEADER_LEN)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(CMD_HEART_BEAT)
	resppkt.SetSeq(pkt.Seq())
	conn.Send(resppkt)
	r.connmgr.UpdateConnActiveTime(conn)
	// r.count++
	// if r.count%2 == 0 {
	// 	conn.Close()
	// }
}

func (r *RequestDispatcher) handleClientClosed(pkt PolePacket, conn *WebSocketConn) {

	r.connmgr.DetachIPAddress(conn)
	r.connmgr.DetachUserFromConn(conn)

	//just process motive close event
	if pkt.Seq() == 1 {
		elog.Info(conn.String(), "proactive close")
		r.connmgr.RelelaseAddress(r.connmgr.GeIPByWebsocketConn(conn))
	}

}
