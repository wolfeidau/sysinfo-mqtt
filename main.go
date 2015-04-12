package main

import (
	"os"
	"os/signal"

	"github.com/alecthomas/kingpin"
	"github.com/juju/loggo"
	"github.com/rcrowley/go-metrics"
)

const statsTopic = "$device/stats"

var (
	debug    = kingpin.Flag("debug", "Enable debug mode.").OverrideDefaultFromEnvar("DEBUG").Bool()
	daemon   = kingpin.Flag("daemon", "Run in daemon mode.").Short('d').Bool()
	mqttURL  = kingpin.Flag("mqttUrl", "The MQTT url to publish too.").Short('u').Default("tcp://localhost:1883").String()
	port     = kingpin.Flag("port", "HTTP Port.").Short('i').OverrideDefaultFromEnvar("PORT").Default("9980").Int()
	interval = kingpin.Flag("interval", "Publish interval.").Short('i').Default("30").Int()

	log = loggo.GetLogger("sysinfo_mqtt")
)

func main() {
	kingpin.Version(Version)
	kingpin.Parse()

	setupLoggo(*debug)

	localRegistry := metrics.NewRegistry()
	publisher := newPublisher(localRegistry)

	engine, err := newSysInfoEngine(publisher)

	if err != nil {
		panic(err)
	}

	ws := newWsServer(localRegistry)

	go ws.listenAndServ(*port)

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	// Wait for receiving a signal.
	<-sigc

	// Disconnect the Network Connection.
	if err := engine.disconnect(); err != nil {
		panic(err)
	}
}

func setupLoggo(debug bool) {
	// apply flags
	if debug {
		loggo.GetLogger("").SetLogLevel(loggo.DEBUG)
	} else {
		loggo.GetLogger("").SetLogLevel(loggo.INFO)
	}
}
