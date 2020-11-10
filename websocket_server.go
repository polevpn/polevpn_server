package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
)

type WebSocketServer struct {
	handler  *RequestDispatcher
	upgrader *websocket.Upgrader
}

func NewWebSocketServer(handler *RequestDispatcher) *WebSocketServer {

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return &WebSocketServer{handler: handler, upgrader: upgrader}
}

func (wss *WebSocketServer) SetRequestDispatcher(handler *RequestDispatcher) {
	wss.handler = handler
}

func (wss *WebSocketServer) Listen(addr string, path string) error {
	http.HandleFunc(path, wss.wsHandler)
	err := http.ListenAndServe(addr, nil)

	if err != nil {
		return err
	}
	return nil
}

func (wss *WebSocketServer) ListenTLS(addr string, certFile string, keyFile string, path string) error {
	http.HandleFunc(path, wss.wsHandler)
	err := http.ListenAndServeTLS(addr, certFile, keyFile, nil)

	if err != nil {
		return err
	}
	return nil
}

func (wss *WebSocketServer) wsHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := wss.upgrader.Upgrade(w, r, nil)
	if err != nil {
		elog.Error("upgrade http request to ws fail", err)
		return
	}
	elog.Info("accpet new ws conn", conn.RemoteAddr().String())
	if wss.handler == nil {
		elog.Error("request dispatcher haven't set")
		return
	}
	wsconn := NewWebSocketConn(conn, wss.handler)
	go wsconn.Read()
	go wsconn.Write()
}
