package main

import (
    "net/http"
	"flag"
	"log"
	"bufio"
	"os"
	"fmt"
	"math/rand"
	"time"
	
    "golang.org/x/net/websocket"
)


var (
	flgClient = flag.Bool("c", false, "Start as a Client.")
	flgServer = flag.Bool("s", false, "Start as a Server.")
)

func client(e int, id string) {
	
	origin := "http://localhost/"
	url := "ws://localhost:12345/" + id
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	
	recv := func() {
		for {
			var msg = make([]byte, 10)
			var n int
			if n, err = ws.Read(msg); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s", msg[:n])
		}
	}
	
	go recv()
	
	for {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		websocket.Message.Send(ws, text)
	}
}

type dd struct {
	xy []*websocket.Conn
}

var cm *dd = &dd{}

// Echo the data received on the WebSocket.
func EchoServer(ws *websocket.Conn) {
	cm.xy = append(cm.xy, ws)
	
	fmt.Printf("Connected from %s ...\n", ws.Request().RemoteAddr)
	
    for {
	    var content string
		err := websocket.Message.Receive(ws, &content)
		if err != nil {
			log.Fatal(err)
		}
    for _, f := range cm.xy {
	    if f != ws {
	   	 websocket.Message.Send(f, content)
	   	}
    }
}}

const abcs = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
func randomId(l int) string {
	rand.Seed(time.Now().UnixNano())
	
	var w []byte
	for j:=0;j < l;j++ {
		w = append(w, abcs[rand.Intn(len(abcs))])
	}
	return string(w)
}

// This example demonstrates a trivial echo server.
func server(id string) {	
    http.Handle("/" + id, websocket.Handler(EchoServer))
    fmt.Printf("Running xpaste server [%s]...\n", id)
	log.Fatal(http.ListenAndServe(":12345", nil))
}

const (
	Echo = iota
	NoEcho
)

func main() {
	flag.Parse()
	
	if *flgClient == *flgServer {
		log.Fatal("You must choose either Client or Server mode.")
	}
	
	var k string
	if len(os.Args) > 2 {
		k = os.Args[2]
	} else {
		k = randomId(6)
	}
	
	if *flgClient {
		client(Echo, k)
		return
	}
	
	if *flgServer {
		go server(k)
		client(NoEcho, k)
		return
	}
}