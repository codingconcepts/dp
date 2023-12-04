package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"github.com/codingconcepts/lb/pkg/list"
)

var (
	selectedServer string
)

func main() {
	var sf stringFlags
	flag.Var(&sf, "server", "a collection of servers to talk to")

	port := flag.Int("port", 26257, "port number to listen on")
	flag.Parse()

	if len(sf) == 0 {
		log.Fatalf("need at least 1 server")
	}

	selectionChanged := make(chan string, 1)

	go handleServerChanged(selectionChanged)

	go func() {
		if err := list.RenderList(sf, selectionChanged); err != nil {
			log.Fatalf("error rendering list: %v", err)
		}
	}()

	proxyAddr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server: %v", err)
	}

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Printf("error accepting client connection: %v", err)
			continue
		}

		go handleClient(client, selectedServer)
	}
}

func handleServerChanged(selectedChanged chan string) {
	for server := range selectedChanged {
		selectedServer = server
	}
}

func handleClient(client net.Conn, server string) {
	defer client.Close()

	tcpServer, err := net.Dial("tcp", server)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
		return
	}
	defer tcpServer.Close()

	go io.Copy(tcpServer, client)
	io.Copy(client, tcpServer)
}

type stringFlags []string

func (sf *stringFlags) String() string {
	sb := strings.Builder{}

	for i, s := range *sf {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, s))
	}

	return sb.String()
}

func (sf *stringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}
