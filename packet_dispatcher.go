package main

import (
	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
)

type PacketDispatcher struct {
	connmgr *WebSocketConnMgr
}

func NewPacketDispatcher() *PacketDispatcher {

	return &PacketDispatcher{}
}

func (p *PacketDispatcher) SetWebSocketConnMgr(connmgr *WebSocketConnMgr) {
	p.connmgr = connmgr
}

func (p *PacketDispatcher) Dispatch(pkt []byte) {

	ipv4pkt := header.IPv4(pkt)

	ipaddr := ipv4pkt.DestinationAddress().To4().String()
	conn := p.connmgr.GetWebSocketConnByIP(ipaddr)
	if conn == nil {
		elog.Debug("connmgr can't find wsconn for", ipaddr)
		return
	}
	buf := make([]byte, len(pkt)+POLE_PACKET_HEADER_LEN)
	copy(buf[POLE_PACKET_HEADER_LEN:], pkt)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(CMD_S2C_IPDATA)
	conn.Send(resppkt)

}
