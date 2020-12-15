package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
	"github.com/polevpn/h2conn"
)

type HttpServer struct {
	wsRequestHandler *WSRequestHandler
	h2RequestHandler *H2RequestHandler
	connMgr          *ConnMgr
	loginchecker     LoginChecker
	upgrader         *websocket.Upgrader
	uplimit          uint64
	downlimit        uint64
}

func NewHttpServer(uplimit uint64, downlimit uint64, wsRequestHandler *WSRequestHandler, h2RequestHandler *H2RequestHandler) *HttpServer {

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return &HttpServer{wsRequestHandler: wsRequestHandler, h2RequestHandler: h2RequestHandler, upgrader: upgrader, uplimit: uplimit, downlimit: downlimit}
}

func (hs *HttpServer) SetLoginCheckHandler(loginchecker LoginChecker) {
	hs.loginchecker = loginchecker
}

func (hs *HttpServer) Listen(addr string, wsPath string, h2Path string) error {
	http.HandleFunc("/", hs.defaultHandler)
	http.HandleFunc(wsPath, hs.wsHandler)
	http.HandleFunc(h2Path, hs.h2Handler)

	err := http.ListenAndServe(addr, nil)

	if err != nil {
		return err
	}
	return nil
}

func (hs *HttpServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
	hs.respError(http.StatusForbidden, w)
}

func (hs *HttpServer) ListenTLS(addr string, certFile string, keyFile string, wsPath string, h2Path string) error {
	http.HandleFunc("/", hs.defaultHandler)
	http.HandleFunc(wsPath, hs.wsHandler)
	http.HandleFunc(h2Path, hs.h2Handler)

	err := http.ListenAndServeTLS(addr, certFile, keyFile, nil)

	if err != nil {
		return err
	}
	return nil
}

func (hs *HttpServer) respError(status int, w http.ResponseWriter) {
	if status == http.StatusBadRequest {
		w.Header().Add("Server", "nginx/1.10.3")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<html>\n<head><title>400 Bad Request</title></head>\n<body bgcolor=\"white\">\n<center><h1>400 Bad Request</h1></center>\n<hr><center>nginx/1.10.3</center>\n</body>\n</html>"))
	} else if status == http.StatusForbidden {
		w.Header().Add("Server", "nginx/1.10.3")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<html>\n<head><title>403 Forbidden</title></head>\n<body bgcolor=\"white\">\n<center><h1>403 Forbidden</h1></center>\n<hr><center>nginx/1.10.3</center>\n</body>\n</html>"))

	}
}

func (hs *HttpServer) h2Handler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	if !hs.loginchecker.CheckLogin(user, pwd) {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
		hs.respError(http.StatusForbidden, w)
		return
	}

	if ip != "" {
		if !hs.h2RequestHandler.connmgr.IsAllocedAddress(ip) {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)

			return
		}

		if hs.h2RequestHandler.connmgr.GetIPAttachUser(ip) != user {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)
			return
		}
	}

	conn, err := h2conn.Accept(w, r)

	if err != nil {
		elog.Error("upgrade http request to h2 fail", err)
		return
	}

	elog.Info("accpet new h2 conn", conn.RemoteAddr().String())

	if hs.h2RequestHandler != nil {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		htpp2conn := NewHttp2Conn(wg, conn, hs.downlimit, hs.uplimit, hs.h2RequestHandler)
		hs.h2RequestHandler.OnConnection(htpp2conn, user, ip)
		go htpp2conn.Read()
		go htpp2conn.Write()
		wg.Wait()
	} else {
		elog.Error("h2 conn handler haven't set")
		conn.Close()
	}

}

func (hs *HttpServer) wsHandler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	if !hs.loginchecker.CheckLogin(user, pwd) {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
		hs.respError(http.StatusForbidden, w)
		return
	}

	if ip != "" {
		if !hs.wsRequestHandler.connmgr.IsAllocedAddress(ip) {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)

			return
		}

		if hs.wsRequestHandler.connmgr.GetIPAttachUser(ip) != user {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)
			return
		}
	}

	conn, err := hs.upgrader.Upgrade(w, r, nil)
	if err != nil {
		elog.Error("upgrade http request to ws fail", err)
		return
	}

	elog.Info("accpet new ws conn", conn.RemoteAddr().String())
	if hs.wsRequestHandler == nil {
		elog.Error("request dispatcher haven't set")
		return
	}

	if hs.wsRequestHandler != nil {
		wsconn := NewWebSocketConn(conn, hs.downlimit, hs.uplimit, hs.wsRequestHandler)
		hs.wsRequestHandler.OnConnection(wsconn, user, ip)
		go wsconn.Read()
		go wsconn.Write()
	} else {
		elog.Error("ws conn handler haven't set")
		conn.Close()
	}

}
