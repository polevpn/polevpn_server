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

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	if user != "polevpn" || pwd != "123456" {
		elog.Error("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if ip != "" {
		if !wss.handler.addresspool.IsAlloc(ip) {
			elog.Error("user:%v,pwd:%v,ip:%v reconnect fail", user, pwd, ip)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

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

	wss.handler.NewConnection(wsconn, ip)
	go wsconn.Read()
	go wsconn.Write()
}
