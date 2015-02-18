package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/net/websocket"
)

// NAME
// xcp - The cross network, cross operating, cross application copy-paste
//
// USAGE
//
// Start xcp on one machine then start on another machine, and copy paste to the xcp commandline
// or open a browser to http://localhost:2975/<name> on a machine running xcp and view the contents
// in a web browser
//
// -p PORT
// -h HTTP PORT
// -n name

// SEQUENCE
// 1. Start up and MultiCast to see if a server is running with <name>
//    a. No server is running with name: Start server
//       I. Check in Random time if another server is running with name
//          i. No, other server, continue running
//          ii. Yes, other server. Stop and connect to running server
//    b. Yes, another server is running. Connect to that server
//

var (
	flgServerPort = flag.String("p", "2975", "The port that is used for the copy-paste socket")
	flgVerbose = flag.Bool("v", true, "Verbose, print out all of the logging.")
)

func init() {
   if !*flgVerbose {
		log.SetOutput(ioutil.Discard)
	}
}

const (
	maxDatagramSize = 8192
	abcs            = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

const (
	Echo = iota
	NoEcho
)

func randomName(le int) string {
	rand.Seed(time.Now().UnixNano())

	var ran []byte
	for i := 0; i < le; i++ {
		ran = append(ran, abcs[rand.Intn(len(abcs))])
	}
	return string(ran)
}

// multicastServer is a small server that accepts multicast requests
// and returns a response if the name requested is the same as the
// server that is running.
func multicastServer(serverName, serverPort string) {
	log.Println("Starting the multicast server...")
	
	iface, err := net.InterfaceByIndex(1)
	if err != nil {
		log.Fatal(err)
	}
	
	multicastAddrs, err := iface.MulticastAddrs()
	if err != nil {
		log.Fatal(err)
	}
	multicastAddr := multicastAddrs[1].String()
	
	mUDPAddr, err := net.ResolveUDPAddr("udp", multicastAddr+":"+serverPort)
	if err != nil {
		log.Fatal(err)
	}

	mUDP, err := net.ListenMulticastUDP("udp", nil, mUDPAddr)
	if err != nil {
		log.Fatal("ListenMulticastUDP failed:", err)
	}

	mUDP.SetReadBuffer(maxDatagramSize)
	for {
		b := make([]byte, maxDatagramSize)
		n, src, err := mUDP.ReadFromUDP(b)
		
		clientData := string(b[:n])
		cd := strings.Split(clientData, ":")
		clientName := cd[0]
		// clientIp := cd[1]
		clientPort, err := strconv.Atoi(cd[2])
		if err != nil {
			log.Fatal("String Conversion to Int failed:", err)
		}

		log.Println(serverName, clientName)
		if serverName == clientName {
			src.Port = clientPort
			
			log.Println(src)
			mUDP.WriteToUDP([]byte(src.String()), src)
		}
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}
	}

}

// multicastClient checks to see if there is a server with the same
// name already running on the network.
func multicastClient(clientName, serverPort string) string {
	iface, err := net.InterfaceByIndex(1)
	if err != nil {
		log.Fatal(err)
	}
	
	multicastAddrs, err := iface.MulticastAddrs()
	if err != nil {
		log.Fatal(err)
	}
	multicastAddr := multicastAddrs[3].String()
	
	mUDPAddr, err := net.ResolveUDPAddr("udp", multicastAddr+":"+serverPort)
	if err != nil {
		log.Fatal(err)
	}

	mUDP, err := net.DialUDP("udp", nil, mUDPAddr)
	if err != nil {
		log.Fatal(err)
	}

	localhostAddrs, err := iface.Addrs()
	if err != nil {
		log.Fatal(err)
	}
	localhostAddr := localhostAddrs[0].String()
	
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	
	for _, address := range addrs {

           // check the address type and if it is not a loopback the display it
           if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
              if ipnet.IP.To4() != nil {
                localhostAddr = ipnet.IP.String()
              }

           }
     }

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(2 * time.Second)
		timeout <- true
	}()
	
	ch := make(chan string, 1)
	go func(rtn chan string){
		// send the name and address to the multicast address
		
		log.Println("Checking for server response...", localhostAddrs, localhostAddr)
		cUDPAddr, err := net.ResolveUDPAddr("udp", localhostAddr+":0")
		if err != nil {
			log.Fatal(err)
		}
		
		cUDP, err := net.ListenUDP("udp", cUDPAddr)
		clientAddress := cUDP.LocalAddr().String()
		if err != nil {
			log.Fatal("ListenFromUDP failed:", err)
		}
		
		log.Println("Sending ping for server discovery...")
		mUDP.Write([]byte(clientName + ":" + clientAddress))
		
		b := make([]byte, maxDatagramSize)
		n, _, err := cUDP.ReadFromUDP(b)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}

		log.Println("Got server response...")
		rtn <- string(b[:n])
	}(ch)
	
	select {
	case v := <-ch:
		return v
	case <-timeout:
		log.Println("No server response...")
		return ""
	}
	
}

func webHandler(serverName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
	    webTempl.Execute(w, map[string]interface{}{"b": serverName, "a": r.Host})
	}
}

func tcpServer(serverName, serverPort string) {
	log.Println("Starting the tcp server...")
	
	webTempl = template.Must(template.New("webs").Parse(webString))
	
	http.Handle("/cmd/"+serverName, websocket.Handler(echoServer))
	http.HandleFunc("/" + serverName, webHandler(serverName))
	fmt.Printf("Running xcp server [%s]...\n", serverName)

	log.Fatal(http.ListenAndServe(":"+serverPort, nil))
}

func tcpClient(serverName, serverPort string) {
	log.Println("Starting the tcp client...")
	/*iface, err := net.InterfaceByIndex(1)
	if err != nil {
		log.Fatal(err)
	}

	localhostAddrs, err := iface.Addrs()
	if err != nil {
		log.Fatal(err)
	}
	localhostAddr := localhostAddrs[0].String()*/
	
	localhostAddr := "localhost"
	
	log.Println("Got:", localhostAddr, serverPort, serverName)
	
	url := "ws://" + localhostAddr + ":" + serverPort + "/cmd/" + serverName
	ws, err := websocket.Dial(url, "", "http://"+localhostAddr+"/")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			var n int
			var data = make([]byte, 255)
			if n, err = ws.Read(data); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s", data[:n])
		}
	}()

	for {
		reader := bufio.NewReader(os.Stdin)
		txt, _ := reader.ReadString('\n')
		websocket.Message.Send(ws, txt)
	}

}

type w struct {
	conn []*websocket.Conn
}

var wss *w = &w{}

// Echo the data received on the WebSocket.
func echoServer(ws *websocket.Conn) {
	fmt.Printf("Connected from %s ...\n", ws.Request().RemoteAddr)

	wss.conn = append(wss.conn, ws)

	for {
		var data string
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			log.Fatal(err)
		}

		// send to all of the other connections but the
		// current one (becuase it's the one sending
		// the data, so it's already there.
		for _, cnx := range wss.conn {
			if cnx != ws {
				websocket.Message.Send(cnx, data)
			}
		}
	}
}

func main() {
	flag.Parse()

	var args = flag.Args()
	var serverName string

	if len(args) > 0 {
		serverName = args[0]
	} else {
		serverName = randomName(6)
	}

	serverPort := *flgServerPort

	serverData := multicastClient(serverName, serverPort)
	if len(serverData) > 0 {
		tcpClient(serverName, serverPort)
	} else {
		log.Println("Starting up as a server...")
		go multicastServer(serverName, serverPort)
		go tcpServer(serverName, serverPort)
		
		log.Println("Sending Client:", serverData)
		tcpClient(serverName, serverPort)
	}
}

var webTempl *template.Template
var webString = `<html>
<head>
<title>xcp</title>
<script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js"></script>
<script type="text/javascript">
    $(function() {

    var conn;
    var msg = $("#msg");
    var log = $("#data");
	var svnm = "{{.a}}";

    function appendLog(msg) {
        var d = log[0]
        var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
        msg.appendTo(log)
        if (doScroll) {
            d.scrollTop = d.scrollHeight - d.clientHeight;
        }
    }
    
    if (window["WebSocket"]) {
        conn = new WebSocket("ws://{{.a}}/cmd/{{.b}}");
        conn.onclose = function(evt) {
            appendLog($("<div><b>Connection closed.</b></div>"))
        }
        conn.onmessage = function(evt) {
            appendLog($("<div/>").text(evt.data))
        }
    } else {
        appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"))
    }
    });
</script>
</head>
<body>
<div id="data"></div>
</body>
</html>`
