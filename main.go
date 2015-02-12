package main

import (
 _   "io"
    "net/http"
	"flag"
	"log"
	"bufio"
	"os"
	"fmt"
	
    "golang.org/x/net/websocket"
)


var (
	flgClient = flag.Bool("c", false, "Start as a Client.")
	flgServer = flag.Bool("s", false, "Start as a Server.")
)

func client(e int) {
	origin := "http://localhost/"
	url := "ws://localhost:12345/echo"
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
		
		// if _, err := ws.Write([]byte(text)); err != nil {
		//	  log.Fatal(err)
		// }
	}
}

type dd struct {
	xy []*websocket.Conn
}

var cm *dd = &dd{}

// Echo the data received on the WebSocket.
func EchoServer(ws *websocket.Conn) {
	cm.xy = append(cm.xy, ws)
	
	fmt.Println("Connected to server ....")
    //io.Copy(ws, io.TeeReader(ws, os.Stdout))
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

// This example demonstrates a trivial echo server.
func server() {
    http.Handle("/echo", websocket.Handler(EchoServer))
    err := http.ListenAndServe(":12345", nil)
    if err != nil {
        panic("ListenAndServe: " + err.Error())
    }
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
	
	if *flgClient {
		client(Echo)
		return
	}
	
	if *flgServer {
		go server()
		client(NoEcho)
		return
	}
}