package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/net/websocket"
)

// NAME
// xcp - The cross application, network and os copy-paste w/ web-browser view
//
// USAGE
//
// Start xcp on one machine then start on another machine, and copy paste to the xcp commandline
// or open a browser to http://localhost:2975/<name> on a machine running xcp and view the contents
// in a web browser
//
// -p TCP PORT
// -v Turn on/off verbose mode logging

// SEQUENCE
// 1. Start up and MultiCast to see if a server is running with <name>
//    a. No server is running with name: Start server
//       I. Check in <random time> if another server is running with name
//          i. No, other server, continue running
//          ii. Yes, other server. Stop and connect to running server
//    b. Yes, another server is running. Connect to that server
//

const (
	maxDatagramSize = 8192
	abcs            = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var (
	flgSvrPort = flag.String("p", "2975", "The port that is used for the copy-paste socket")
	flgVerbose = flag.Bool("v", false, "Verbose, print out all of the logging.")
)

// randomName returns random characters of n length
func randomName(n int) string {
	rand.Seed(time.Now().UnixNano())

	rtn := make([]byte, n)
	for i := 0; i < n; i++ {
		rtn[i] = abcs[rand.Intn(len(abcs))]
	}

	return string(rtn)
}

// printDot prints a row of dots on a 100ms timer until a signal to stop.
// note the function should be called as a go routine and closes all of the
// channels for you.
func printDot(stop, cont chan struct{}) {
	defer func() {
		close(stop)
		close(cont)
	}()

	if *flgVerbose {
		var hasPrint bool
		for {
			select {
			case <-stop:
				if hasPrint {
					fmt.Print("\n")
				}
				cont <- struct{}{}
				return
			case <-time.After(100 * time.Millisecond):
				hasPrint = true
				fmt.Print(".")
			}
		}
	} else {
		<-stop
		cont <- struct{}{}
	}
}

// multicastServer forwards data from the client to all other connected clients
func multicastServer(svrName, svrPort string) {
	log.Println("Starting the multicast server...")

	var multicastAddr string

	mInterfaces, err := net.Interfaces()
	if err != nil {
		log.Fatal("Parsing network interfaces failed:", err)
	}

	for _, m := range mInterfaces {
		mAddrs, err := m.MulticastAddrs()
		if err != nil {
			log.Fatal("Finding multicast addresses failed:", err)
		}

		for _, mAddr := range mAddrs {
			if ipAddr, ok := mAddr.(*net.IPAddr); ok && ipAddr.IP.IsMulticast() {
				if ipAddr.IP.To4() != nil {
					multicastAddr = ipAddr.IP.String()
				}
			}
		}
	}

	mUDPAddr, err := net.ResolveUDPAddr("udp", multicastAddr+":"+svrPort)
	if err != nil {
		log.Fatal("ResolveUDPAddr failed:", err)
	}

	mUDP, err := net.ListenMulticastUDP("udp", nil, mUDPAddr)
	if err != nil {
		log.Fatal("ListenMulticastUDP failed:", err)
	}

	mUDP.SetReadBuffer(maxDatagramSize)
	for {
		b := make([]byte, maxDatagramSize)
		n, src, err := mUDP.ReadFromUDP(b)
		if err != nil {
			log.Fatal("Read from UDP failed:", err)
		}

		clientData := string(b[:n])

		splitClientData := strings.Split(clientData, ":")
		clientName := splitClientData[0]
		// clientIp := splitClientData[1]
		clientPort, err := strconv.Atoi(splitClientData[2])
		if err != nil {
			log.Fatal("String Conversion to Int failed:", err)
		}

		if svrName == clientName {
			src.Port = clientPort
			mUDP.WriteToUDP([]byte(src.String()), src)
		} else {
			log.Println("Saw ", clientName, "...")
		}
	}
}

// multicastClient sends data to the multicastServer and receives data
// from the 'castServer
func multicastClient(clientName, svrPort string) bool {
	log.Println("Starting the multicast client...")

	var localhostAddr, multicastAddr string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}

	for _, addr := range addrs {
		// check the address type and if it is not a loopback then display it
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localhostAddr = ipnet.IP.String()
			}
		}
	}

	mInterfaces, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	for _, mm := range mInterfaces {
		mAddrs, err := mm.MulticastAddrs()
		if err != nil {
			log.Fatal(err)
		}
		for _, addr := range mAddrs {
			if ipnet, ok := addr.(*net.IPAddr); ok && ipnet.IP.IsMulticast() {
				if ipnet.IP.To4() != nil {
					multicastAddr = ipnet.IP.String()
				}
			}
		}
	}

	mUDPAddr, err := net.ResolveUDPAddr("udp", multicastAddr+":"+svrPort)
	if err != nil {
		log.Fatal(err)
	}

	mUDP, err := net.DialUDP("udp", nil, mUDPAddr)
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan string, 1)
	stp, cot := make(chan struct{}), make(chan struct{})
	go func(rtn chan string) {
		// send the name and address to the multicast address

		log.Println("Checking for server response...", localhostAddr)
		cUDPAddr, err := net.ResolveUDPAddr("udp", localhostAddr+":0")
		if err != nil {
			log.Fatal(err)
		}

		cUDP, err := net.ListenUDP("udp", cUDPAddr)
		clientAddress := cUDP.LocalAddr().String()
		if err != nil {
			log.Fatal("ListenFromUDP failed:", err)
		}

		log.Printf("Sending ping for server %s:%s discovery...", clientName, clientAddress)
		mUDP.Write([]byte(clientName + ":" + clientAddress))

		// show .'s while waiting to see if there is a multicast client``
		go printDot(stp, cot)

		b := make([]byte, maxDatagramSize)
		n, _, err := cUDP.ReadFromUDP(b)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}
		stp <- struct{}{}
		<-cot

		log.Println("Got server response...")
		rtn <- string(b[:n])
	}(ch)

	select {
	case v := <-ch:
		return len(v) > 0
	case <-time.After(2 * time.Second):
		stp <- struct{}{}
		<-cot
		log.Println("No server response...")
		return false
	}
}

// tcpServer receives data from the client and serves to all other clients and the websocket
func tcpServer(svrName, svrPort string) {
	log.Println("Starting the tcp server...")

	webTpl = template.Must(template.New("webs").Parse(webStr))

	http.Handle("/cmd/"+svrName, websocket.Handler(socketHandler))
	http.HandleFunc("/"+svrName, webHandler(svrName))

	fmt.Printf("Running xcp server [%s]...\n", svrName)
	log.Fatal(http.ListenAndServe(":"+svrPort, nil))
}

// tcpClient is the client that will receive data from the server... note one
// machine will have a server and client running.
func tcpClient(svrName, svrPort string) {
	log.Println("Starting the tcp client...")

	url := "ws://localhost:" + svrPort + "/cmd/" + svrName
	ws, err := websocket.Dial(url, "", "http://localhost/")
	if err != nil {
		log.Fatal("Connect to websocket failed:", err)
	}

	go func() {
		for {
			var n, data = 0, make([]byte, 255)
			if n, err = ws.Read(data); err != nil {
				log.Fatal("Read from websocket failed:", err)
			}
			fmt.Printf("%s", data[:n])
		}
	}()

	fmt.Printf("Running xcp client [%s]...\n", svrName)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		websocket.Message.Send(ws, scanner.Text()+"\n")
	}
}

// webHandler serves the webpage that will inspect the xcp traffic
func webHandler(svrName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		webTpl.Execute(w, map[string]interface{}{"name": svrName, "host": r.Host})
	}
}

// socketHandler sets up the websocket between the web and the tcp socket
func socketHandler(ws *websocket.Conn) {
	fmt.Printf("Connected from %s ...\n", ws.Request().RemoteAddr)

	wss.conn = append(wss.conn, ws)

	for {
		var data string
		err := websocket.Message.Receive(ws, &data)
		if err == io.EOF {
			return
		} else if err != nil {
			log.Fatal("Receiving from websocket failed:", err)
		}

		// send to all of the other connections but the
		// current one, because it's the one sending
		// the data, so it's already there.
		for _, cx := range wss.conn {
			if cx != ws {
				websocket.Message.Send(cx, data)
			}
		}
	}
}

func main() {
	flag.Parse()

	if !*flgVerbose {
		log.SetOutput(ioutil.Discard)
	}

	var args = flag.Args()
	var svrName string

	if len(args) > 0 {
		svrName = args[0]
	} else {
		svrName = randomName(6)
	}

	svrPort := *flgSvrPort
	hasMulticast := multicastClient(svrName, svrPort)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for range sig {
			fmt.Printf("\nClosing the client and server [%s].\n", svrName)
			os.Exit(0)
		}
	}()

	if hasMulticast {
		log.Println("Starting up as a client only...")
		tcpClient(svrName, svrPort)
	} else {
		log.Println("Starting up as a server...")
		go multicastServer(svrName, svrPort)
		go tcpServer(svrName, svrPort)

		tcpClient(svrName, svrPort)
	}
}

type wsc struct {
	conn []*websocket.Conn
}

var wss *wsc = &wsc{}

var webTpl *template.Template
var webStr = `<html>
	<head>
		<title>xcp - display</title>
		<link href="https://fonts.googleapis.com/css?family=Lato" rel="stylesheet">
		<link rel="stylesheet" type="text/css" href="https://cdnjs.cloudflare.com/ajax/libs/meyer-reset/2.0/reset.min.css" >
		<style>
			body { padding: 10px; }
			#data {
				color: #434343;
				font-family: 'Lato', sans-serif;
				font-weight: 100;
				font-size: 1em;
			}
		</style>
		<script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js"></script>
		<script type="text/javascript">
			$(function() {

				var conn,
					msg = $("#msg"),
					data = $("#data");

				function fmt(data) {
					if (typeof data == 'string') {
						var s = data.split("\n");
						
						if (s[s.length - 1] == "") {
							s.pop();
						}
						if (s.length > 1) {
							data = "<div>" + s.join("</div><div>") + "</div>";
						} else {
							data = s[0];
						}
						
						data = data.replace(/\t/g, '\u00a0\u00a0\u00a0\u00a0');
						data = data.replace(/\s/g, '\u00a0');
					}
					return data
				}

				function appendLog(msg) {
					var d = data[0]
					var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
					msg.appendTo(data)
					if (doScroll) {
						d.scrollTop = d.scrollHeight - d.clientHeight;
					}
				}
			
				if (window["WebSocket"]) {
					conn = new WebSocket("ws://{{.host}}/cmd/{{.name}}");
					conn.onclose = function(evt) {
						appendLog($("<div><b>Connection closed.</b></div>"));
					}
					conn.onmessage = function(evt) {
						appendLog($("<div/>").html(fmt(evt.data)));
					}
				} else {
					appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"))
				}
		
				// Paste into browser
				// Copied from: https://blog.dmbcllc.com/cross-browser-javascript-copy-and-paste/
				// (c) Dave Bush
				var systemPasteReady = false;
				var systemPasteContent;
				var textArea;
				
				function paste(target) {

					if (window.clipboardData) {
						target.innerText = window.clipboardData.getData('Text');
						return;
					}
					function waitForPaste() {
						if (!systemPasteReady) {
							setTimeout(waitForPaste, 250);
							return;
						}
						target.innerHTML = systemPasteContent;
						systemPasteReady = false;
						document.body.removeChild(textArea);
						textArea = null;
					}
					// FireFox requires at least one editable 
					// element on the screen for the paste event to fire
					textArea = document.createElement('textarea');
					textArea.setAttribute('style', 'width:1px;border:0;opacity:0;');
					document.body.appendChild(textArea);
					textArea.select();
					
					waitForPaste();
				}

				function systemPasteListener(evt) {
					systemPasteContent = evt.clipboardData.getData('text/plain');
					systemPasteReady = true;
					evt.preventDefault();
					conn.send(systemPasteContent+"\n");
					appendLog($("<div/>").html(fmt(systemPasteContent)));
				}

				function keyBoardListener(evt) {
					if (evt.ctrlKey) {
						switch(evt.keyCode) {
							case 86: // v
								paste(evt.target);
								break;
						}
					}
				}

				window.addEventListener('paste', systemPasteListener);
				document.addEventListener('keydown', keyBoardListener);
			});
		</script>
	</head>
	<body>
		<div id="data"></div>
	</body>
</html>`
