package main

import (
	"io"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
)

const (
	CH_WEBSOCKET_WRITE_SIZE = 2048
)

type WebSocketConn struct {
	conn      *websocket.Conn
	wch       chan []byte
	closed    bool
	handler   *RequestDispatcher
	downlimit uint64
	uplimit   uint64
	tfcounter *TrafficCounter
}

func NewWebSocketConn(conn *websocket.Conn, downlimit uint64, uplimit uint64, handler *RequestDispatcher) *WebSocketConn {
	return &WebSocketConn{
		conn:      conn,
		closed:    false,
		wch:       make(chan []byte, CH_WEBSOCKET_WRITE_SIZE),
		handler:   handler,
		downlimit: downlimit,
		uplimit:   uplimit,
		tfcounter: NewTrafficCounter(),
	}
}

func (wsc *WebSocketConn) Close(flag bool) error {
	if wsc.closed == false {
		wsc.closed = true
		if wsc.wch != nil {
			wsc.wch <- nil
			close(wsc.wch)
		}
		err := wsc.conn.Close()
		if flag {
			pkt := make([]byte, POLE_PACKET_HEADER_LEN)
			PolePacket(pkt).SetCmd(CMD_CLIENT_CLOSED)
			PolePacket(pkt).SetSeq(0)
			go wsc.handler.Dispatch(pkt, wsc)
		}
		return err
	}
	return nil
}

func (wsc *WebSocketConn) String() string {
	return wsc.conn.LocalAddr().String() + "->" + wsc.conn.RemoteAddr().String()
}

func (wsc *WebSocketConn) IsClosed() bool {
	return wsc.closed
}

func (wsc *WebSocketConn) Read() {
	defer func() {
		wsc.Close(true)
	}()

	defer PanicHandler()

	for {
		mtype, pkt, err := wsc.conn.ReadMessage()
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				elog.Info(wsc.String(), "conn closed")
			} else {
				elog.Error(wsc.String(), "conn read exception:", err)
			}
			return
		}
		if mtype == websocket.BinaryMessage {

			//traffic limit
			ppkt := PolePacket(pkt)
			if ppkt.Cmd() == CMD_C2S_IPDATA {
				bytes, ltime := wsc.tfcounter.UPStreamCount(uint64(len(ppkt.Payload())))
				if bytes > wsc.uplimit/(1000/TRAFFIC_LIMIT_INTERVAL) {
					duration := ltime.Add(time.Millisecond * TRAFFIC_LIMIT_INTERVAL).Sub(time.Now())
					if duration > 0 {
						drop := false
						if len(wsc.wch) > CH_WEBSOCKET_WRITE_SIZE*0.75 {
							ippkt := header.IPv4(ppkt.Payload())
							if ippkt.Protocol() == uint8(tcp.ProtocolNumber) {
								n := rand.Intn(5)
								if n > 2 {
									drop = true
								}
							} else if ippkt.Protocol() == uint8(udp.ProtocolNumber) {
								udppkt := header.UDP(ippkt.Payload())
								if udppkt.DestinationPort() != 53 {
									drop = true
								}
							}
						}

						if drop {
							continue
						} else {
							time.Sleep(duration)
						}
					}
				}
			}
			wsc.handler.Dispatch(pkt, wsc)
		}
	}

}

func (wsc *WebSocketConn) Write() {
	defer PanicHandler()

	for {

		pkt, ok := <-wsc.wch
		if !ok {
			elog.Error(wsc.String(), "get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info(wsc.String(), "exit write process")
			return
		}

		//traffic limit
		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			bytes, ltime := wsc.tfcounter.DownStreamCount(uint64(len(ppkt.Payload())))
			if bytes > wsc.downlimit/(1000/TRAFFIC_LIMIT_INTERVAL) {
				duration := ltime.Add(time.Millisecond * TRAFFIC_LIMIT_INTERVAL).Sub(time.Now())
				if duration > 0 {
					drop := false
					if len(wsc.wch) > CH_WEBSOCKET_WRITE_SIZE*0.75 {
						ippkt := header.IPv4(ppkt.Payload())
						if ippkt.Protocol() == uint8(tcp.ProtocolNumber) {
							n := rand.Intn(5)
							if n > 2 {
								drop = true
							}
						} else if ippkt.Protocol() == uint8(udp.ProtocolNumber) {
							udppkt := header.UDP(ippkt.Payload())
							if udppkt.SourcePort() != 53 {
								drop = true
							}
						}
					}

					if drop {
						continue
					} else {
						time.Sleep(duration)
					}
				}
			}
		}

		err := wsc.conn.WriteMessage(websocket.BinaryMessage, pkt)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				elog.Info(wsc.String(), "conn closed")
			} else {
				elog.Error(wsc.String(), "conn write exception:", err)
			}
			return
		}

	}
}

func (wsc *WebSocketConn) Send(pkt []byte) {
	if wsc.closed == true {
		return
	}
	if wsc.wch != nil {
		wsc.wch <- pkt
	}
}
