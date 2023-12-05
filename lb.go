package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

const (
	drainOption = "drain"
)

var (
	selectedServerMu sync.RWMutex
	selectedServer   string

	terminateSignalMu sync.Mutex
	terminateSignal   chan struct{}
)

func main() {
	var sf stringFlags
	flag.Var(&sf, "server", "a collection of servers to talk to")

	port := flag.Int("port", 26257, "port number to listen on")
	forceClose := flag.Bool("force", true, "force close connections when server changes")
	flag.Parse()

	if len(sf) == 0 {
		log.Fatalf("need at least 1 server")
	}

	availableServers := sf.toMap()
	selectedServer = availableServers[0]
	terminateSignal = make(chan struct{})

	go inputLoop(availableServers, *forceClose)

	proxyAddr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server: %v", err)
	}

	for {
		if err = accept(listener); err != nil {
			log.Printf("error in accept: %v", err)
		}
	}
}

func accept(listener net.Listener) error {
	client, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting client connection: %w", err)
	}

	selectedServerMu.RLock()
	defer selectedServerMu.RUnlock()

	if selectedServer == "" || selectedServer == drainOption {
		client.Close()
		return nil
	}

	go handleClient(client, selectedServer)
	return nil
}

func inputLoop(availableServers map[int]string, forceClose bool) {
	for {
		//fmt.Println("\033[H\033[2J")
		fmt.Println(availableServersString(availableServers))
		fmt.Printf("Selected: %s\n", selectedServer)
		fmt.Printf("\n> ")

		var input string
		if _, err := fmt.Scan(&input); err != nil {
			log.Printf("error reading input: %v", err)
			continue
		}

		selection, err := strconv.Atoi(input)
		if err != nil {
			continue
		}

		selectedServerMu.Lock()
		selectedServer = availableServers[selection]
		selectedServerMu.Unlock()

		if forceClose {
			terminateSignalMu.Lock()
			close(terminateSignal)
			terminateSignal = make(chan struct{})
			terminateSignalMu.Unlock()
		}
	}
}

func handleClient(client net.Conn, server string) {
	defer client.Close()

	tcpServer, err := dial(client, server)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
		return
	}
	defer tcpServer.Close()

	go io.Copy(tcpServer, client)
	go io.Copy(client, tcpServer)

	// Wait for server to change and allow function to complete (and connection
	// to close) when it does.
	<-terminateSignal
}

func dial(client net.Conn, server string) (net.Conn, error) {
	if _, ok := client.(*tls.Conn); ok {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}

		return tls.Dial("tcp", server, tlsConfig)
	}

	return net.Dial("tcp", server)
}

type stringFlags []string

func availableServersString(m map[int]string) string {
	sb := strings.Builder{}

	for i := 0; i < len(m); i++ {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i, m[i]))
	}

	return sb.String()
}

func (sf *stringFlags) String() string {
	return availableServersString(sf.toMap())
}

func (sf *stringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

func (sf *stringFlags) toMap() map[int]string {
	m := map[int]string{
		0: drainOption,
	}

	for i, server := range *sf {
		m[i+1] = server
	}

	return m
}
