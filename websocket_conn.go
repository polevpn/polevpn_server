package main

import (
	"io"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
)

type WebSocketConn struct {
	conn    *websocket.Conn
	wch     chan []byte
	closed  bool
	handler *RequestDispatcher
}

func NewWebSocketConn(conn *websocket.Conn, handler *RequestDispatcher) *WebSocketConn {
	return &WebSocketConn{conn: conn, closed: false, wch: make(chan []byte, 100), handler: handler}
}

func (wsc *WebSocketConn) Close() error {
	if wsc.closed == false {
		wsc.closed = true
		return wsc.conn.Close()
	}
	return nil
}

func (wsc *WebSocketConn) IsClosed() bool {
	return wsc.closed
}

func (wsc *WebSocketConn) Read() {
	defer func() {
		wsc.Close()
		if wsc.wch != nil {
			wsc.wch <- nil
			close(wsc.wch)
		}
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
			return
		}
		if mtype != websocket.BinaryMessage {
			continue
		}

		go wsc.handler.Dispatch(pkt, wsc)

	}

}

func (wsc *WebSocketConn) Write() {
	defer PanicHandler()

	for {
		select {
		case pkt, ok := <-wsc.wch:
			if !ok {
				elog.Error("get pkt from write channel fail,maybe channel closed")
				return
			} else {
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
	}
}

func (wsc *WebSocketConn) Send(pkt []byte) {
	if wsc.closed == true {
		elog.Info("websocket connection is closed,can't send pkt")
		return
	}
	if wsc.wch != nil {
		wsc.wch <- pkt
	}
}
