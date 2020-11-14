package main

import (
	"io"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
)

const (
	CH_WEBSOCKET_WRITE_SIZE = 2048
)

type WebSocketConn struct {
	conn    *websocket.Conn
	wch     chan []byte
	closed  bool
	handler *RequestDispatcher
}

func NewWebSocketConn(conn *websocket.Conn, handler *RequestDispatcher) *WebSocketConn {
	return &WebSocketConn{conn: conn, closed: false, wch: make(chan []byte, CH_WEBSOCKET_WRITE_SIZE), handler: handler}
}

func (wsc *WebSocketConn) Close() error {
	if wsc.closed == false {
		wsc.closed = true
		if wsc.wch != nil {
			wsc.wch <- nil
			close(wsc.wch)
		}
		return wsc.conn.Close()
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
		wsc.Close()
	}()

	defer PanicHandler()

	for {
		mtype, pkt, err := wsc.conn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				elog.Info(wsc.conn.LocalAddr().String(), wsc.conn.RemoteAddr().String(), "conn closed")
			} else {
				elog.Error(wsc.conn.LocalAddr().String(), wsc.conn.RemoteAddr().String(), "conn read exception:", err)
			}
			pkt = make([]byte, POLE_PACKET_HEADER_LEN)
			PolePacket(pkt).SetCmd(CMD_CLIENT_CLOSED)
			PolePacket(pkt).SetSeq(0)
			go wsc.handler.Dispatch(pkt, wsc)
			return
		}
		if mtype == websocket.BinaryMessage {
			wsc.handler.Dispatch(pkt, wsc)
		} else if mtype == websocket.CloseMessage {
			pkt = make([]byte, POLE_PACKET_HEADER_LEN)
			PolePacket(pkt).SetCmd(CMD_CLIENT_CLOSED)
			PolePacket(pkt).SetSeq(1)
			go wsc.handler.Dispatch(pkt, wsc)
		}

	}

}

func (wsc *WebSocketConn) Write() {
	defer PanicHandler()

	for {

		pkt, ok := <-wsc.wch
		if !ok {
			elog.Error("get pkt from write channel fail,maybe channel closed")
			return
		}
		if pkt == nil {
			elog.Info("exit write process")
			return
		}
		err := wsc.conn.WriteMessage(websocket.BinaryMessage, pkt)
		if err != nil {
			if err == io.EOF {
				elog.Info(wsc.conn.LocalAddr().String(), wsc.conn.RemoteAddr().String(), "conn closed")
			} else {
				elog.Error(wsc.conn.LocalAddr().String(), wsc.conn.RemoteAddr().String(), "conn write exception:", err)
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
