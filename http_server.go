package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/polevpn/elog"
	"github.com/polevpn/h2conn"
	"github.com/polevpn/xnet/http2"
	"github.com/polevpn/xnet/http2/h2c"
)

type HttpServer struct {
	requestHandler *RequestHandler
	connMgr        *ConnMgr
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
	}

	return &HttpServer{requestHandler: requestHandler, upgrader: upgrader, uplimit: uplimit, downlimit: downlimit}
}

func (hs *HttpServer) SetLoginCheckHandler(loginchecker LoginChecker) {
	hs.loginchecker = loginchecker
}

func (hs *HttpServer) Listen(wg *sync.WaitGroup, addr string, wsPath string, h2Path string, hcPath string) {

	defer wg.Done()

	h2s := &http2.Server{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == wsPath {
			hs.wsHandler(w, r)
		} else if r.URL.Path == h2Path {
			hs.h2Handler(w, r)
		} else if r.URL.Path == hcPath {
			hs.hcHandler(w, r)
		} else {
			hs.defaultHandler(w, r)
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, h2s),
	}

	elog.Error(server.ListenAndServe())

}

func (hs *HttpServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
	hs.respError(http.StatusForbidden, w)
}

func (hs *HttpServer) ListenTLS(wg *sync.WaitGroup, addr string, certFile string, keyFile string, wsPath string, h2Path string, hcPath string) {

	defer wg.Done()

	http.HandleFunc("/", hs.defaultHandler)
	http.HandleFunc(wsPath, hs.wsHandler)
	http.HandleFunc(h2Path, hs.h2Handler)
	http.HandleFunc(hcPath, hs.hcHandler)

	elog.Error(http.ListenAndServeTLS(addr, certFile, keyFile, nil))
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

func (hs *HttpServer) hcHandler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	if r.Method == http.MethodPut {

		user := r.URL.Query().Get("user")
		pwd := r.URL.Query().Get("pwd")
		ip := r.URL.Query().Get("ip")

		elog.Infof("http user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

		if !hs.loginchecker.CheckLogin(user, pwd) {
			elog.Errorf("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
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
		streamId := strconv.FormatInt(time.Now().UnixNano(), 10)
		streamId = strings.Split(r.RemoteAddr, ":")[0] + "-" + streamId
		w.Write([]byte(streamId))
		conn := NewHttpConn(streamId, hs.downlimit, hs.uplimit, hs.requestHandler)
		hs.requestHandler.connmgr.SetConn(streamId, conn)
		conn.handler.OnConnection(conn, user, ip)

	} else if r.Method == http.MethodGet {
		streamId := r.URL.Query().Get("stream")
		conn := hs.requestHandler.connmgr.GetConn(streamId)

		if conn == nil {
			hs.respError(http.StatusForbidden, w)
			return
		}

		httpconn := conn.(*HttpConn)
		flusher := w.(http.Flusher)
		httpconn.SetDownStream(w, flusher)
		w.Header().Add("Content-Length", strconv.FormatInt(10*1024*1024*1024, 10))
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		httpconn.Write()

	} else if r.Method == http.MethodPost {
		streamId := r.URL.Query().Get("stream")
		conn := hs.requestHandler.connmgr.GetConn(streamId)

		if conn == nil {
			hs.respError(http.StatusForbidden, w)
			return
		}

		httpconn := conn.(*HttpConn)
		httpconn.SetUpStream(r.Body)
		httpconn.Read()
	}

}

func (hs *HttpServer) h2Handler(w http.ResponseWriter, r *http.Request) {

	defer PanicHandler()

	user := r.URL.Query().Get("user")
	pwd := r.URL.Query().Get("pwd")
	ip := r.URL.Query().Get("ip")

	elog.Infof("http2 user:%v,pwd:%v,ip:%v connect", user, pwd, ip)

	if !hs.loginchecker.CheckLogin(user, pwd) {
		elog.Errorf("user:%v,pwd:%v,ip:%v verify fail", user, pwd, ip)
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

	conn, err := h2conn.Accept(w, r)

	if err != nil {
		elog.Error("upgrade http request to h2 fail", err)
		return
	}

	elog.Info("accpet new h2 conn", conn.RemoteAddr().String())

	if hs.requestHandler != nil {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		htpp2conn := NewHttp2Conn(wg, conn, hs.downlimit, hs.uplimit, hs.requestHandler)
		hs.requestHandler.OnConnection(htpp2conn, user, ip)
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
		elog.Error("upgrade http request to ws fail", err)
		return
	}

	elog.Info("accpet new ws conn", conn.RemoteAddr().String())
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
