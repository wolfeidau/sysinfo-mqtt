package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/juju/loggo"
	"github.com/rcrowley/go-metrics"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

var (
	debug    = kingpin.Flag("debug", "Enable debug mode.").OverrideDefaultFromEnvar("DEBUG").Bool()
	daemon   = kingpin.Flag("daemon", "Run in daemon mode.").Short('d').Bool()
	mqttURL  = kingpin.Flag("mqttUrl", "The MQTT url to publish too.").Short('u').Default("tcp://localhost:1883").String()
	interval = kingpin.Flag("interval", "Publish interval.").Short('i').Default("30").Int()

	log           = loggo.GetLogger("sysinfo_mqtt")
	localRegistry = metrics.NewRegistry()
)

func main() {
	kingpin.Version(Version)
	kingpin.Parse()

	setupLoggo(*debug)

	murl, err := url.Parse(*mqttURL)

	if err != nil {
		panic(err)
	}

	// Create an MQTT Client.
	cli := client.New(&client.Options{
		ErrorHandler: func(err error) {
			fmt.Println(err)
		},
	})

	// Connect to the MQTT Server.
	err = cli.Connect(&client.ConnectOptions{
		Network:  murl.Scheme,
		Address:  murl.Host,
		ClientID: []byte("sysinfo-mqtt"),
	})

	if err != nil {
		panic(err)
	}

	p := newPublisher(localRegistry)

	ticker := time.NewTicker(time.Second * 5)

	go func() {

		for t := range ticker.C {
			log.Infof("Tick at %v", t)
			p.flush()
		}

	}()

	dumpTicker := time.NewTicker(time.Second * 30)

	go func() {

		for t := range dumpTicker.C {

			metrics := exportMetrics(localRegistry)

			payload, _ := json.Marshal(struct {
				Time    int64
				Payload map[string]interface{}
			}{
				Time:    t.Unix(),
				Payload: metrics,
			})

			cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte("$device/stats"),
				Message:   payload,
			})
		}

	}()

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	// Wait for receiving a signal.
	<-sigc

	// Disconnect the Network Connection.
	if err := cli.Disconnect(); err != nil {
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
