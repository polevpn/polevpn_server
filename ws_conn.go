package main

import (
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
	"github.com/polevpn/netstack/tcpip/header"
	"github.com/polevpn/netstack/tcpip/transport/tcp"
	"github.com/polevpn/netstack/tcpip/transport/udp"
)

const (
	CH_WEBSOCKET_WRITE_SIZE = 100
	TRAFFIC_LIMIT_INTERVAL  = 10
)

type WebSocketConn struct {
	conn         *websocket.Conn
	wch          chan []byte
	closed       bool
	handler      *RequestHandler
	downlimit    uint64
	uplimit      uint64
	tcDownStream *TrafficCounter
	tcUpStream   *TrafficCounter
}

func NewWebSocketConn(conn *websocket.Conn, downlimit uint64, uplimit uint64, handler *RequestHandler) *WebSocketConn {
	return &WebSocketConn{
		conn:         conn,
		closed:       false,
		wch:          make(chan []byte, CH_WEBSOCKET_WRITE_SIZE),
		handler:      handler,
		downlimit:    downlimit,
		uplimit:      uplimit,
		tcDownStream: NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
		tcUpStream:   NewTrafficCounter(TRAFFIC_LIMIT_INTERVAL * time.Millisecond),
	}
}

func (wsc *WebSocketConn) Close(flag bool) error {
	if !wsc.closed {
		wsc.closed = true
		if wsc.wch != nil {
			wsc.wch <- nil
			close(wsc.wch)
		}
		err := wsc.conn.Close()
		if flag {
			go wsc.handler.OnClosed(wsc, false)
		}
		return err
	}
	return nil
}

func (wsc *WebSocketConn) String() string {
	return wsc.conn.RemoteAddr().String() + "->" + wsc.conn.LocalAddr().String()
}

func (wsc *WebSocketConn) IsClosed() bool {
	return wsc.closed
}

func (wsc *WebSocketConn) checkStreamLimit(pkt []byte, tfcounter *TrafficCounter, limit uint64) (bool, time.Duration) {
	bytes, ltime := tfcounter.StreamCount(uint64(len(pkt)))
	if bytes > limit/(1000/uint64(tfcounter.StreamCountInterval()/time.Millisecond)) {
		duration := ltime.Add(tfcounter.StreamCountInterval()).Sub(time.Now())
		if duration > 0 {
			drop := false
			if len(wsc.wch) > CH_WEBSOCKET_WRITE_SIZE*0.5 {
				ippkt := header.IPv4(pkt)
				if ippkt.Protocol() == uint8(tcp.ProtocolNumber) {
					n := rand.Intn(5)
					if n > 2 {
						drop = true
					}
				} else if ippkt.Protocol() == uint8(udp.ProtocolNumber) {
					udppkt := header.UDP(ippkt.Payload())
					if udppkt.DestinationPort() != 53 && udppkt.SourcePort() != 53 {
						drop = true
					}
				}
			}

			if drop {
				return true, 0
			} else {
				return true, duration
			}
		}
	}
	return false, 0
}

func (wsc *WebSocketConn) Read() {
	defer func() {
		wsc.Close(true)
	}()

	defer PanicHandler()

	for {
		mtype, pkt, err := wsc.conn.ReadMessage()
		if err != nil {

			elog.Error(wsc.String(), " wsconn read end,status=", err)
			return
		}
		if mtype == websocket.BinaryMessage {

			ppkt := PolePacket(pkt)
			if ppkt.Cmd() == CMD_C2S_IPDATA {
				//traffic limit
				limit, duration := wsc.checkStreamLimit(ppkt.Payload(), wsc.tcUpStream, wsc.uplimit)
				if limit {
					if duration > 0 {
						time.Sleep(duration)
					} else {
						continue
					}
				}
			}
			wsc.handler.OnRequest(pkt, wsc)
		} else {
			elog.Info("ws mtype=", mtype)
		}
	}

}

func (wsc *WebSocketConn) drainWriteCh() {
	for {
		select {
		case _, ok := <-wsc.wch:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func (wsc *WebSocketConn) Write() {
	defer PanicHandler()
	defer wsc.drainWriteCh()

	for {

		pkt, ok := <-wsc.wch
		if !ok {
			elog.Error(wsc.String(), " channel closed")
			return
		}
		if pkt == nil {
			elog.Info(wsc.String(), " exit write process")
			return
		}

		ppkt := PolePacket(pkt)
		if ppkt.Cmd() == CMD_S2C_IPDATA {
			//traffic limit
			limit, duration := wsc.checkStreamLimit(ppkt.Payload(), wsc.tcDownStream, wsc.downlimit)
			if limit {
				if duration > 0 {
					time.Sleep(duration)
				} else {
					continue
				}
			}
		}
		err := wsc.conn.WriteMessage(websocket.BinaryMessage, pkt)
		if err != nil {
			elog.Error(wsc.String(), " wsconn write end,status=", err)
			return
		}
	}
}

func (wsc *WebSocketConn) Send(pkt []byte) {
	if wsc.closed {
		return
	}
	if wsc.wch != nil {
		select {
		case wsc.wch <- pkt:
		default:
			elog.Error(wsc.String(), " wch is full")
		}
	}
}
