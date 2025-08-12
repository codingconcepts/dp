package server

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/codingconcepts/errhandler"
	"github.com/rs/zerolog"
)

type Server struct {
	httpPort    int
	connections int64
	logger      zerolog.Logger

	portGroupsMu     sync.RWMutex
	portGroups       map[int]map[string]group
	terminateSignals map[int]chan struct{}
}

type group struct {
	Weight  float64  `json:"weight"`
	Servers []string `json:"servers"`
}

func New(logger zerolog.Logger, httpPort int, ports ...int) *Server {
	s := Server{
		httpPort:         httpPort,
		logger:           logger,
		terminateSignals: make(map[int]chan struct{}),
		portGroups:       map[int]map[string]group{},
	}

	// Initialize port groups and terminate signals for each port.
	for _, port := range ports {
		s.portGroups[port] = make(map[string]group)
		s.terminateSignals[port] = make(chan struct{}, 1)
	}

	return &s
}

func (svr *Server) PortListen(wg *sync.WaitGroup, p int) error {
	defer wg.Done()

	proxyAddr := fmt.Sprintf("localhost:%d", p)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server on port %d: %v", p, err)
	}
	defer listener.Close()

	svr.logger.Info().Int("port", p).Msg("listening")

	for {
		if err = svr.accept(listener, p); err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				svr.logger.Debug().Int("port", p).Msg("listener closed")
				return nil
			}
			svr.logger.Err(err).Int("port", p).Msg("")
		}
	}
}

func (svr *Server) accept(listener net.Listener, port int) error {
	svr.logger.Debug().Str("action", "connect").Str("addr", listener.Addr().String()).Int("port", port).Msg("")
	defer svr.logger.Debug().Str("action", "disconnect").Str("addr", listener.Addr().String()).Int("port", port).Msg("")

	client, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting client connection: %w", err)
	}

	server := svr.selectServerByWeight(port)
	if server == "" {
		client.Close()
		return nil
	}

	go svr.handleClient(client, server, port)
	return nil
}

func (svr *Server) selectServerByWeight(port int) string {
	svr.portGroupsMu.RLock()
	defer svr.portGroupsMu.RUnlock()

	portGroup, exists := svr.portGroups[port]
	if !exists {
		return ""
	}

	var activeGroups []struct {
		name   string
		weight float64
	}
	var totalWeight float64

	for name, group := range portGroup {
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

	// Select group based on weight.
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
			Int("port", port).
			Msg("no group selected")
	}

	// Randomly select a server from the chosen group.
	servers := portGroup[selectedGroup].Servers
	if len(servers) == 0 {
		svr.logger.Fatal().
			Any("servers", portGroup[selectedGroup].Servers).
			Str("group", selectedGroup).
			Int("port", port).
			Msg("no servers available")
	}

	return servers[rand.Intn(len(servers))]
}

func (svr *Server) handleClient(client net.Conn, server string, port int) {
	tcpServer, err := net.Dial("tcp", server)
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
	<-svr.terminateSignals[port]
	atomic.AddInt64(&svr.connections, -1)
}

func (svr *Server) HTTPServer(port int) {
	m := http.NewServeMux()

	m.Handle("GET /ports", errhandler.Wrap(svr.handleGetPorts))
	m.Handle("GET /ports/{port}/groups", errhandler.Wrap(svr.handleGetGroups))
	m.Handle("POST /ports/{port}/groups", errhandler.Wrap(svr.handleSetGroup))
	m.Handle("DELETE /ports/{port}/group/{group}", errhandler.Wrap(svr.handleDeleteGroup))
	m.Handle("POST /ports/{port}/activate", errhandler.Wrap(svr.handleActivation))

	s := &http.Server{
		Handler: m,
		Addr:    fmt.Sprintf(":%d", port),
	}

	log.Fatal(s.ListenAndServe())
}

func (svr *Server) handleGetPorts(w http.ResponseWriter, r *http.Request) error {
	svr.logger.Info().Str("action", "get ports").Msg("started")
	defer svr.logger.Info().Str("action", "get ports").Msg("finished")

	svr.portGroupsMu.RLock()
	defer svr.portGroupsMu.RUnlock()

	return errhandler.SendJSON(w, svr.portGroups)
}

func (svr *Server) handleGetGroups(w http.ResponseWriter, r *http.Request) error {
	port, err := svr.parsePort(r)
	if err != nil {
		return errhandler.Error(http.StatusBadRequest, err)
	}

	svr.logger.Info().Str("action", "get groups").Int("port", port).Msg("started")
	defer svr.logger.Info().Str("action", "get groups").Int("port", port).Msg("finished")

	svr.portGroupsMu.RLock()
	defer svr.portGroupsMu.RUnlock()

	portGroup, exists := svr.portGroups[port]
	if !exists {
		return errhandler.SendJSON(w, map[string]group{})
	}

	return errhandler.SendJSON(w, portGroup)
}

type setGroupRequest struct {
	Name    string   `json:"name"`
	Servers []string `json:"servers"`
	Weight  float64  `json:"weight"`
}

func (svr *Server) handleSetGroup(w http.ResponseWriter, r *http.Request) error {
	port, err := svr.parsePort(r)
	if err != nil {
		return errhandler.Error(http.StatusBadRequest, err)
	}

	svr.logger.Info().Str("action", "set groups").Int("port", port).Msg("started")
	defer svr.logger.Info().Str("action", "set groups").Int("port", port).Msg("finished")

	var req setGroupRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	log.Printf("[SET] port: %d group: %q servers: %v weight: %.2f", port, req.Name, req.Servers, req.Weight)

	svr.setGroupServers(port, req.Name, req.Servers, req.Weight)

	return nil
}

func (svr *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) error {
	port, err := svr.parsePort(r)
	if err != nil {
		return errhandler.Error(http.StatusBadRequest, err)
	}

	svr.logger.Info().Str("action", "delete groups").Int("port", port).Msg("started")
	defer svr.logger.Info().Str("action", "delete groups").Int("port", port).Msg("finished")

	group := r.PathValue("group")

	svr.deleteGroup(port, group)

	return nil
}

type activationRequest struct {
	Groups  []string  `json:"groups"`
	Weights []float64 `json:"weights"`
}

func (svr *Server) handleActivation(w http.ResponseWriter, r *http.Request) error {
	port, err := svr.parsePort(r)
	if err != nil {
		return errhandler.Error(http.StatusBadRequest, err)
	}

	svr.logger.Info().Str("action", "activate").Int("port", port).Msg("started")
	defer svr.logger.Info().Str("action", "activate").Int("port", port).Msg("finished")

	var req activationRequest
	if err := errhandler.ParseJSON(r, &req); err != nil {
		return errhandler.Error(http.StatusUnprocessableEntity, err)
	}

	svr.setActiveGroups(port, req.Groups, req.Weights)

	// Close and recreate the terminate signal for this port
	close(svr.terminateSignals[port])
	svr.terminateSignals[port] = make(chan struct{})

	return nil
}

func (svr *Server) deleteGroup(port int, group string) {
	svr.portGroupsMu.Lock()
	defer svr.portGroupsMu.Unlock()

	if portGroup, exists := svr.portGroups[port]; exists {
		delete(portGroup, group)
	}
}

func (svr *Server) setGroupServers(port int, g string, servers []string, weight float64) {
	svr.portGroupsMu.Lock()
	defer svr.portGroupsMu.Unlock()

	// Ensure the port group exists
	if _, exists := svr.portGroups[port]; !exists {
		svr.portGroups[port] = make(map[string]group)
	}

	if foundGroup, ok := svr.portGroups[port][g]; ok {
		foundGroup.Servers = servers
		if weight > 0 {
			foundGroup.Weight = weight
		}
		svr.portGroups[port][g] = foundGroup
	} else {
		svr.portGroups[port][g] = group{
			Weight:  weight,
			Servers: servers,
		}
	}
}

func (svr *Server) setActiveGroups(port int, groups []string, weights []float64) {
	svr.portGroupsMu.Lock()
	defer svr.portGroupsMu.Unlock()

	// Ensure the port group exists
	if _, exists := svr.portGroups[port]; !exists {
		svr.portGroups[port] = make(map[string]group)
	}

	// Set all weights to 0 for this port
	for k, v := range svr.portGroups[port] {
		v.Weight = 0
		svr.portGroups[port][k] = v
	}

	var found bool
	var totalWeight float64

	for i, g := range groups {
		weight := float64(0)
		if i < len(weights) {
			weight = weights[i]
		}

		if foundGroup, ok := svr.portGroups[port][g]; ok {
			svr.logger.Info().Int("port", port).Str("group", g).Float64("weight", weight).Msg("")

			if weight > 0 {
				foundGroup.Weight = weight
			}
			svr.portGroups[port][g] = foundGroup

			totalWeight += foundGroup.Weight
			found = true
		}
	}

	if !found {
		svr.logger.Info().Int("port", port).Msg("drained")
	}
}

func (svr *Server) parsePort(r *http.Request) (int, error) {
	portStr := r.PathValue("port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %s", portStr)
	}
	return port, nil
}
