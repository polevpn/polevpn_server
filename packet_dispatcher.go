package main

import (
	"net"

	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
)

const (
	IPV4_PROTOCOL = 4
	IPV6_PROTOCOL = 6
)

type PacketDispatcher struct {
	connmgr   *ConnMgr
	routermgr *RouterMgr
}

func NewPacketDispatcher() *PacketDispatcher {

	return &PacketDispatcher{}
}

func (p *PacketDispatcher) SetConnMgr(connmgr *ConnMgr) {
	p.connmgr = connmgr
}

func (p *PacketDispatcher) SetRouterMgr(routermgr *RouterMgr) {
	p.routermgr = routermgr
}

func (p *PacketDispatcher) Dispatch(pkt []byte) {

	ver := pkt[0]
	ver = ver >> 4
	if ver != IPV4_PROTOCOL {
		return
	}

	ipv4pkt := header.IPv4(pkt)

	ip := ipv4pkt.DestinationAddress().To4()
	ipstr := ip.String()
	conn := p.connmgr.GetConnByIP(ipstr)

	if conn == nil {
		gw := p.routermgr.FindRoute(net.IP(ip))
		conn = p.connmgr.GetConnByIP(gw)
	}

	if conn == nil {
		elog.Debug("connmgr can't find wsconn for", ipstr)
		return
	}
	buf := make([]byte, len(pkt)+POLE_PACKET_HEADER_LEN)
	copy(buf[POLE_PACKET_HEADER_LEN:], pkt)
	resppkt := PolePacket(buf)
	resppkt.SetLen(uint16(len(buf)))
	resppkt.SetCmd(CMD_S2C_IPDATA)
	conn.Send(resppkt)

}
