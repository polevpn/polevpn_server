package main

import (
	"github.com/google/netstack/tcpip/header"
	_ "github.com/google/netstack/tcpip/header"
	"github.com/polevpn/elog"
)

type PacketDispatcher struct {
	dch     chan []byte
	connmgr *WebSocketConnMgr
}

func NewPacketDispatcher(size int, connmgr *WebSocketConnMgr) *PacketDispatcher {

	return &PacketDispatcher{dch: make(chan []byte, size), connmgr: connmgr}
}

func (p *PacketDispatcher) Dispatch(pkt []byte) {
	// if p.dch != nil {
	// 	p.dch <- pkt
	// }
	ipv4pkt := header.IPv4(pkt)

	ipaddr := ipv4pkt.DestinationAddress().To4().String()
	conn := p.connmgr.GetWebSocketConn(ipaddr)
	if conn == nil {
		elog.Info("connmgr can find wsconn for", ipaddr)
	}
	buf := make([]byte, len(pkt)+POLE_PACKET_HEADER_LEN)
	copy(buf[POLE_PACKET_HEADER_LEN:], pkt)
	resppkt := PolePacket(buf)
	resppkt.SetCmd(CMD_S2C_IPDATA)
	conn.Send(resppkt)

}

func (p *PacketDispatcher) Process() {

}
