package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/polevpn/elog"
	"github.com/polevpn/h3conn"
)

const (
	TCP_WRITE_BUFFER_SIZE = 524288
	TCP_READ_BUFFER_SIZE  = 524288
)

type HttpServer struct {
	requestHandler *RequestHandler
	loginchecker   LoginChecker
	upgrader       *websocket.Upgrader
	uplimit        uint64
	downlimit      uint64
}

func NewHttpServer(uplimit uint64, downlimit uint64, requestHandler *RequestHandler) *HttpServer {

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: false,
	}

	return &HttpServer{requestHandler: requestHandler, upgrader: upgrader, uplimit: uplimit, downlimit: downlimit}
}

func (hs *HttpServer) SetLoginCheckHandler(loginchecker LoginChecker) {
	hs.loginchecker = loginchecker
}

func (hs *HttpServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
	hs.respError(http.StatusForbidden, w)
}

func (hs *HttpServer) ListenTLS(wg *sync.WaitGroup, addr string, certFile string, keyFile string) {

	defer wg.Done()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/" {
			if r.ProtoAtLeast(3, 0) {
				hs.h3Handler(w, r)
			} else {
				hs.wsHandler(w, r)
			}
		} else {
			hs.defaultHandler(w, r)
		}
	})
	elog.Error(http3.ListenAndServe(addr, certFile, keyFile, handler))
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

func (hs *HttpServer) h3Handler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	if user == "" || pwd == "" {
		hs.respError(http.StatusForbidden, w)
		return
	}

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	err := hs.loginchecker.CheckLogin(user, pwd)
	if err != nil {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail,%v", user, pwd, ip, err)
		hs.respError(http.StatusForbidden, w)
		return
	}

	if ip != "" {
		if !hs.requestHandler.connmgr.IsAllocedAddress(ip) {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)

			return
		}

		if hs.requestHandler.connmgr.GetIPAttachUser(ip) != user {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)
			return
		}
	}

	conn, err := h3conn.Accept(w, r)
	if err != nil {
		elog.Error("upgrade http request to h3 fail", err)
		return
	}

	elog.Info("accpet new h3 conn ", conn.RemoteAddr().String())
	if hs.requestHandler == nil {
		elog.Error("request dispatcher haven't set")
		return
	}

	if hs.requestHandler != nil {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		h3conn := NewHttp3Conn(wg, conn, hs.downlimit, hs.uplimit, hs.requestHandler)
		hs.requestHandler.OnConnection(h3conn, user, ip)
		go h3conn.Read()
		go h3conn.Write()
		wg.Wait()
	} else {
		elog.Error("h3 conn handler haven't set")
		conn.Close()
	}

}

func (hs *HttpServer) wsHandler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	if user == "" || pwd == "" {
		hs.respError(http.StatusForbidden, w)
		return
	}

	elog.Infof("user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	err := hs.loginchecker.CheckLogin(user, pwd)
	if err != nil {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail,%v", user, pwd, ip, err)
		hs.respError(http.StatusForbidden, w)
		return
	}

	if ip != "" {
		if !hs.requestHandler.connmgr.IsAllocedAddress(ip) {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not alloc to it", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)

			return
		}

		if hs.requestHandler.connmgr.GetIPAttachUser(ip) != user {
			elog.Errorf("user:%v,pwd:%v,ip:%v reconnect fail,ip address not belong to the user", user, pwd, ip)
			hs.respError(http.StatusBadRequest, w)
			return
		}
	}

	conn, err := hs.upgrader.Upgrade(w, r, nil)
	if err != nil {
		elog.Error("upgrade http request to ws fail,", err)
		return
	}

	elog.Info("accpet new ws conn ", conn.RemoteAddr().String())
	if hs.requestHandler == nil {
		elog.Error("request dispatcher haven't set")
		return
	}

	if hs.requestHandler != nil {
		wsconn := NewWebSocketConn(conn, hs.downlimit, hs.uplimit, hs.requestHandler)
		hs.requestHandler.OnConnection(wsconn, user, ip)
		go wsconn.Read()
		go wsconn.Write()
	} else {
		elog.Error("ws conn handler haven't set")
		conn.Close()
	}

}
