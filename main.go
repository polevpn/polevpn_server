package main

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/gorilla/websocket"
	"github.com/songgao/water"
)

//sudo ifconfig utun4 10.1.0.1 10.1.0.2 up

var (
	upgrader = websocket.Upgrader{
		//允许跨域访问
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

var ifce *water.Interface

func createTunInterface() (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	var err error
	ifce, err = water.New(config)
	if err != nil {
		return nil, err
	}

	// ip addr add dev tun0 local 10.8.0.1 peer 10.8.0.2
	// ip route add 10.8.0.0/24 via 10.8.0.2
	_, err = exec.Command("bash", "-c", "ip addr add dev "+ifce.Name()+" local 10.8.0.1 peer 10.8.0.2").Output()

	if err != nil {
		return nil, err
	}

	_, err = exec.Command("bash", "-c", "sudo ip link set "+ifce.Name()+" up").Output()

	if err != nil {
		return nil, err
	}

	_, err = exec.Command("bash", "-c", "ip route add 10.0.0.0/8 via 10.8.0.2").Output()

	if err != nil {
		return nil, err
	}

	return ifce, nil
}

func webSocketRead(conn *websocket.Conn) {
	defer conn.Close()

	for {
		_, pkg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("websocket read data error", err)
			break
		}

		ipv4Pkg := NewIPv4Header()
		ipv4Pkg.Unmarshal(pkg)
		fmt.Printf("Outbound Pkg %s\n", ipv4Pkg.String())

		// srcIP := net.IPv4(10, 8, 0, 7)

		// data, err := ipv4Pkg.WithSource(srcIP).WithCSum(0).Marshal()
		// fmt.Printf("Outbound2 Pkg %s\n", ipv4Pkg.String())

		_, err = ifce.Write(pkg)

		if err != nil {
			fmt.Println("tun write fail", err)
			break
		}

	}

}

func webSocketWrite(conn *websocket.Conn) {

	defer conn.Close()

	var pkg []byte = make([]byte, 65535)

	for {

		n, err := ifce.Read(pkg)
		if err != nil {
			fmt.Println("tun read fail", err)
		}
		pkg1 := pkg[:n]

		ipv4Pkg := NewIPv4Header()

		ipv4Pkg.Unmarshal(pkg)
		fmt.Printf("Inbound Pkg %s\n", ipv4Pkg.String())

		err = conn.WriteMessage(websocket.BinaryMessage, pkg1)
		if err != nil {
			fmt.Println("websocket write fail", err)
			break
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("accpet new conn", conn.RemoteAddr().String())
	go webSocketRead(conn)
	go webSocketWrite(conn)
}

func main() {

	var err error

	ifce, err = createTunInterface()
	if err != nil {
		fmt.Println("create tun fail", err)
		return
	}
	http.HandleFunc("/ws", wsHandler)
	err = http.ListenAndServe("0.0.0.0:8080", nil)

	if err != nil {
		fmt.Println("http listen error", err)
	}
}
