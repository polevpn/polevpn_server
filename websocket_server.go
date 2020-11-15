package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
)

type WebSocketServer struct {
	handler      *RequestDispatcher
	loginchecker LoginChecker
	upgrader     *websocket.Upgrader
	uplimit      uint64
	downlimit    uint64
}

func NewWebSocketServer(uplimit uint64, downlimit uint64, handler *RequestDispatcher) *WebSocketServer {

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return &WebSocketServer{handler: handler, upgrader: upgrader, uplimit: uplimit, downlimit: downlimit}
}

func (wss *WebSocketServer) SetLoginCheckHandler(loginchecker LoginChecker) {
	wss.loginchecker = loginchecker
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

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	if !wss.loginchecker.CheckLogin(user, pwd) {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if ip != "" {
		if !wss.handler.connmgr.IsAllocedAddress(ip) {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if wss.handler.connmgr.GetIPAttachUser(ip) != user {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
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

	if wss.handler != nil {
		wsconn := NewWebSocketConn(conn, wss.downlimit, wss.uplimit, wss.handler)
		wss.handler.NewConnection(wsconn, user, ip)
		go wsconn.Read()
		go wsconn.Write()
	} else {
		elog.Error("ws conn handler haven't set")
		conn.Close()
	}

}
