package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/codingconcepts/errhandler"
	"github.com/samber/lo"
)

var (
	version string
)

func main() {
	log.SetFlags(0)
	port := flag.Int("port", 26257, "port number for proxy requests")
	ctlPort := flag.Int("ctl-port", 3000, "port number for proxy control requests")
	showVersion := flag.Bool("version", false, "show the application version")
	debug := flag.Bool("debug", false, "enable debug-level logging")
	flag.Parse()

	if *showVersion {
		log.Printf("dp version %s", version)
		return
	}

	svr := server{
		httpPort:        *ctlPort,
		terminateSignal: make(chan struct{}, 1),
		serverGroups:    map[string]group{},
		debug:           *debug,
	}

	go svr.httpServer(*ctlPort)

	proxyAddr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server: %v", err)
	}

	for {
		if err = svr.accept(listener); err != nil {
			log.Printf("error in accept: %v", err)
		}
	}
}

type server struct {
	httpPort    int
	connections int64
	debug       bool

	serversMu    sync.RWMutex
	serverGroups map[string]group

	terminateSignal chan struct{}
}

type group struct {
	Active  bool     `json:"active"`
	Servers []string `json:"servers"`
}

func (svr *server) accept(listener net.Listener) error {
	client, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting client connection: %w", err)
	}

	servers := svr.activeServers()

	if len(servers) == 0 {
		client.Close()
		return nil
	}

	server := lo.Sample(servers)
	if svr.debug {
		fmt.Printf("server: %s\n", server)
	}

	go svr.handleClient(client, server)
	return nil
}

func (svr *server) handleClient(client net.Conn, server string) {
	tcpServer, err := dial(client, server)
	if err != nil {
		// Error will be obvious from connected clients.
		return
	}

	// Ensure the client and server are closed.
	defer tcpServer.Close()
	defer client.Close()

	go io.Copy(tcpServer, client)
	go io.Copy(client, tcpServer)

	// Wait for server to change and allow function to complete (and connection
	// to close) when it does.
	atomic.AddInt64(&svr.connections, 1)
	<-svr.terminateSignal
	atomic.AddInt64(&svr.connections, -1)
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

func (svr *server) httpServer(port int) {
	m := http.NewServeMux()

	m.Handle("GET /groups", errhandler.Wrap(svr.handleGetGroups))
	m.Handle("POST /groups", errhandler.Wrap(svr.handleSetGroup))
	m.Handle("DELETE /groups/{group}", errhandler.Wrap(svr.handleDeleteGroup))
	m.Handle("POST /activate", errhandler.Wrap(svr.handleActivation))

	s := &http.Server{
		Handler: m,
		Addr:    fmt.Sprintf(":%d", port),
	}

	log.Fatal(s.ListenAndServe())
}

func (svr *server) handleGetGroups(w http.ResponseWriter, r *http.Request) error {
	svr.serversMu.RLock()
	defer svr.serversMu.RUnlock()

	return errhandler.SendJSON(w, svr.serverGroups)
}

type setGroupRequest struct {
	Name    string   `json:"name"`
	Servers []string `json:"servers"`
}

func (svr *server) handleSetGroup(w http.ResponseWriter, r *http.Request) error {
	var req setGroupRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	log.Printf("[SET] group: %q servers: %v", req.Name, req.Servers)

	svr.setGroupServers(req.Name, req.Servers)

	return nil
}

func (svr *server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) error {
	group := r.PathValue("group")

	svr.deleteGroup(group)

	return nil
}

type activationRequest struct {
	Groups []string `json:"groups"`
}

func (svr *server) handleActivation(w http.ResponseWriter, r *http.Request) error {
	var req activationRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	svr.setActiveGroups(req.Groups)

	close(svr.terminateSignal)
	svr.terminateSignal = make(chan struct{})

	return nil
}

func (svr *server) deleteGroup(group string) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	// Delete group.
	delete(svr.serverGroups, group)
}

func (svr *server) setGroupServers(g string, servers []string) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	if foundGroup, ok := svr.serverGroups[g]; ok {
		foundGroup.Servers = servers
		svr.serverGroups[g] = foundGroup
	} else {
		svr.serverGroups[g] = group{
			Active:  false,
			Servers: servers,
		}
	}
}

func (svr *server) setActiveGroups(groups []string) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	// Disable all groups.
	for k, v := range svr.serverGroups {
		v.Active = false
		svr.serverGroups[k] = v
	}

	// Enable given groups.
	for _, g := range groups {
		if foundGroup, ok := svr.serverGroups[g]; ok {
			foundGroup.Active = true
			svr.serverGroups[g] = foundGroup
		}
	}
}

func (svr *server) activeServers() []string {
	svr.serversMu.RLock()
	defer svr.serversMu.RUnlock()

	var servers []string

	for _, group := range svr.serverGroups {
		if group.Active {
			servers = append(servers, group.Servers...)
		}
	}

	return servers
}
