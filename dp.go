package main

import (
	"flag"
	"log"
	"os"
	"sync"

	"github.com/codingconcepts/dp/pkg/models"
	"github.com/codingconcepts/dp/pkg/server"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

var (
	version string
)

func main() {
	log.SetFlags(0)

	var ports models.IntFlags
	flag.Var(&ports, "port", "port number for proxy requests (can be specified multiple times)")
	ctlPort := flag.Int("ctl-port", 3000, "port number for proxy control requests")
	showVersion := flag.Bool("version", false, "show the application version")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	// Validate flags.
	if len(ports) == 0 || *ctlPort == 0 {
		flag.Usage()
		os.Exit(2)
	}

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

	svr := server.New(logger, *ctlPort, ports...)

	// Listen for control requests.
	go svr.HTTPServer(*ctlPort)

	// Listen on each of the provided ports.
	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go svr.PortListen(&wg, port)
	}
	wg.Wait()
}
