package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/codingconcepts/errhandler"
	"github.com/rs/zerolog"
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
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	logger := zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stderr,
		PartsExclude: []string{
			zerolog.TimestampFieldName,
		},
	}).Level(lo.Ternary(*verbose, zerolog.DebugLevel, zerolog.InfoLevel))

	if *showVersion {
		logger.Info().Str("version", version).Msg("")
		return
	}

	svr := server{
		httpPort:        *ctlPort,
		logger:          logger,
		terminateSignal: make(chan struct{}, 1),
		serverGroups:    map[string]group{},
	}

	go svr.httpServer(*ctlPort)

	proxyAddr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server: %v", err)
	}

	log.Printf("ready")

	for {
		if err = svr.accept(listener); err != nil {
			log.Printf("error in accept: %v", err)
		}
	}
}

type server struct {
	httpPort    int
	connections int64
	logger      zerolog.Logger

	serversMu    sync.RWMutex
	serverGroups map[string]group

	terminateSignal chan struct{}
}

type group struct {
	Weight  float64  `json:"weight"`
	Servers []string `json:"servers"`
}

func (svr *server) accept(listener net.Listener) error {
	svr.logger.Debug().Str("action", "connect").Str("addr", listener.Addr().String()).Msg("")
	defer svr.logger.Debug().Str("action", "connect").Str("addr", listener.Addr().String()).Msg("")

	client, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting client connection: %w", err)
	}

	server := svr.selectServerByWeight()
	if server == "" {
		client.Close()
		return nil
	}

	go svr.handleClient(client, server)
	return nil
}

func (svr *server) selectServerByWeight() string {
	svr.serversMu.RLock()
	defer svr.serversMu.RUnlock()

	var activeGroups []struct {
		name   string
		weight float64
	}
	var totalWeight float64

	for name, group := range svr.serverGroups {
		if group.Weight > 0 && len(group.Servers) > 0 {
			activeGroups = append(activeGroups, struct {
				name   string
				weight float64
			}{name, group.Weight})
			totalWeight += group.Weight
		}
	}

	if len(activeGroups) == 0 {
		return ""
	}

	// Select group based on weight
	r := rand.Float64() * totalWeight
	var cumulativeWeight float64
	var selectedGroup string

	for _, g := range activeGroups {
		cumulativeWeight += g.weight
		if r <= cumulativeWeight {
			selectedGroup = g.name
			break
		}
	}

	// If we didn't select a group log an error.
	if selectedGroup == "" {
		svr.logger.Fatal().
			Any("groups", activeGroups).
			Float64("total_weight", totalWeight).
			Msg("no group selected")
	}

	// Now randomly select a server from the chosen group
	servers := svr.serverGroups[selectedGroup].Servers
	if len(servers) == 0 {
		svr.logger.Fatal().
			Any("servers", svr.serverGroups[selectedGroup].Servers).
			Str("group", selectedGroup).
			Msg("no servers available")
	}

	return servers[rand.Intn(len(servers))]
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
	svr.logger.Info().Str("action", "get groups").Msg("started")
	defer svr.logger.Info().Str("action", "get groups").Msg("finished")

	svr.serversMu.RLock()
	defer svr.serversMu.RUnlock()

	return errhandler.SendJSON(w, svr.serverGroups)
}

type setGroupRequest struct {
	Name    string   `json:"name"`
	Servers []string `json:"servers"`
	Weight  float64  `json:"weight"`
}

func (svr *server) handleSetGroup(w http.ResponseWriter, r *http.Request) error {
	svr.logger.Info().Str("action", "set groups").Msg("started")
	defer svr.logger.Info().Str("action", "set groups").Msg("finished")

	var req setGroupRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	log.Printf("[SET] group: %q servers: %v weight: %.2f", req.Name, req.Servers, req.Weight)

	svr.setGroupServers(req.Name, req.Servers, req.Weight)

	return nil
}

func (svr *server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) error {
	svr.logger.Info().Str("action", "delete groups").Msg("started")
	defer svr.logger.Info().Str("action", "delete groups").Msg("finished")

	group := r.PathValue("group")

	svr.deleteGroup(group)

	return nil
}

type activationRequest struct {
	Groups  []string  `json:"groups"`
	Weights []float64 `json:"weights"`
}

func (svr *server) handleActivation(w http.ResponseWriter, r *http.Request) error {
	svr.logger.Info().Str("action", "activate").Msg("started")
	defer svr.logger.Info().Str("action", "activate").Msg("finished")

	var req activationRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	svr.setActiveGroups(req.Groups, req.Weights)

	close(svr.terminateSignal)
	svr.terminateSignal = make(chan struct{})

	return nil
}

func (svr *server) deleteGroup(group string) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	delete(svr.serverGroups, group)
}

func (svr *server) setGroupServers(g string, servers []string, weight float64) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	if foundGroup, ok := svr.serverGroups[g]; ok {
		foundGroup.Servers = servers
		if weight > 0 {
			foundGroup.Weight = weight
		}
		svr.serverGroups[g] = foundGroup
	} else {
		svr.serverGroups[g] = group{
			Weight:  weight,
			Servers: servers,
		}
	}
}

func (svr *server) setActiveGroups(groups []string, weights []float64) {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	for k, v := range svr.serverGroups {
		v.Weight = 0
		svr.serverGroups[k] = v
	}

	var found bool
	var totalWeight float64

	for i, g := range groups {
		weight := float64(0)
		if i < len(weights) {
			weight = weights[i]
		}

		if foundGroup, ok := svr.serverGroups[g]; ok {
			svr.logger.Info().Str("group", g).Float64("weight", weight).Msg("")

			if weight > 0 {
				foundGroup.Weight = weight
			}
			svr.serverGroups[g] = foundGroup

			totalWeight += foundGroup.Weight
			found = true
		}
	}

	if !found {
		svr.logger.Info().Msg("drained")
	}
}
