package main

import (
	"net"

	"github.com/polevpn/anyvalue"
	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
)

type RequestHandler struct {
	tunio     *TunIO
	connmgr   *ConnMgr
	routermgr *RouterMgr
}

func NewRequestHandler() *RequestHandler {

	return &RequestHandler{}
}

func (r *RequestHandler) SetTunIO(tunio *TunIO) {
	r.tunio = tunio
}

func (r *RequestHandler) SetConnMgr(connmgr *ConnMgr) {
	r.connmgr = connmgr
}

func (r *RequestHandler) SetRouterMgr(routermgr *RouterMgr) {
	r.routermgr = routermgr
}

func (r *RequestHandler) OnRequest(pkt []byte, conn Conn) {

	ppkt := PolePacket(pkt)
	switch ppkt.Cmd() {
	case CMD_ALLOC_IPADDR:
		elog.Info("received alloc ip adress request from ", conn.String())
		r.handleAllocIPAddress(ppkt, conn)
	case CMD_C2S_IPDATA:
		r.handleC2SIPData(ppkt, conn)
	case CMD_HEART_BEAT:
		elog.Debug("received heart beat request,", conn.String())
		r.handleHeartBeat(ppkt, conn)
	case CMD_CLIENT_CLOSED:
		r.handleClientClose(ppkt, conn)
	default:
		elog.Error("invalid pkt cmd=", ppkt.Cmd())
	}
}

func (r *RequestHandler) OnConnection(conn Conn, user string, ip string) {
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

func (r *RequestHandler) handleAllocIPAddress(pkt PolePacket, conn Conn) {
	av := anyvalue.New()

	ip := r.connmgr.AllocAddress(conn)

	if ip == "" {
		elog.Error("ip alloc fail,no more ip address")
	}

	elog.Infof("alloc ip %v to %v", ip, conn.String())
	av.Set("ip", ip)
	av.Set("dns", Config.Get("dns").AsStr())
	av.Set("route", Config.Get("client_routes").AsStrArr())
	body, _ := av.MarshalJSON()
	buf := make([]byte, POLE_PACKET_HEADER_LEN+len(body))
	copy(buf[POLE_PACKET_HEADER_LEN:], body)
	resppkt := PolePacket(buf)
	resppkt.SetLen(uint16(len(buf)))
	resppkt.SetCmd(pkt.Cmd())
	conn.Send(resppkt)
	if ip != "" {
		r.connmgr.AttachIPAddressToConn(ip, conn)
		r.connmgr.AttachUserToIP(r.connmgr.GetConnAttachUser(conn), ip)
	}
}

func (r *RequestHandler) handleC2SIPData(pkt PolePacket, conn Conn) {

	ipv4pkg := header.IPv4(pkt.Payload())
	dstIp := ipv4pkg.DestinationAddress().To4()
	dstIpStr := dstIp.String()

	elog.Debug("received pkt to ", dstIpStr)

	toconn := r.connmgr.GetConnByIP(dstIpStr)

	if toconn == nil {
		gw := r.routermgr.FindRoute(net.IP(dstIp))
		toconn = r.connmgr.GetConnByIP(gw)
	}

	if toconn != nil {
		pkt.SetCmd(CMD_S2C_IPDATA)
		toconn.Send(pkt)
	} else {
		if r.tunio != nil {
			err := r.tunio.Enqueue(pkt[POLE_PACKET_HEADER_LEN:])
			if err != nil {
				elog.Error("tunio enqueue fail,", err)
			}
		}
	}
}

func (r *RequestHandler) handleHeartBeat(pkt PolePacket, conn Conn) {
	buf := make([]byte, POLE_PACKET_HEADER_LEN)
	resppkt := PolePacket(buf)
	resppkt.SetLen(POLE_PACKET_HEADER_LEN)
	resppkt.SetCmd(CMD_HEART_BEAT)
	conn.Send(resppkt)
	r.connmgr.UpdateConnActiveTime(conn)
}

func (r *RequestHandler) handleClientClose(pkt PolePacket, conn Conn) {
	elog.Info(conn.String(), " proactive close")
	r.connmgr.RelelaseAddress(r.connmgr.GeIPByConn(conn))
}

func (r *RequestHandler) OnClosed(conn Conn, proactive bool) {

	elog.Info("connection closed event from ", conn.String())

	r.connmgr.DetachIPAddressFromConn(conn)
	r.connmgr.DetachUserFromConn(conn)
	//just process proactive close event
	if proactive {
		elog.Info(conn.String(), " proactive close")
		r.connmgr.RelelaseAddress(r.connmgr.GeIPByConn(conn))
	}

}
