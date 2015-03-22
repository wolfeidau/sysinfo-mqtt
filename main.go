package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/armon/go-metrics"
	"github.com/juju/loggo"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

var (
	debug    = kingpin.Flag("debug", "Enable debug mode.").OverrideDefaultFromEnvar("DEBUG").Bool()
	daemon   = kingpin.Flag("daemon", "Run in daemon mode.").Short('d').Bool()
	mqttURL  = kingpin.Flag("mqttUrl", "The MQTT url to publish too.").Short('u').Default("tcp://localhost:1883").String()
	interval = kingpin.Flag("interval", "Publish interval.").Short('i').Default("30").Int()

	log = loggo.GetLogger("sysinfo_mqtt")
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

	inmem := metrics.NewInmemSink(10*time.Second, time.Minute)
	pub := NewInmemPublish(inmem, 30*time.Second, func(metrics map[string]interface{}) {

		var data bytes.Buffer
		enc := gob.NewEncoder(&data)

		err := enc.Encode(metrics)

		if err != nil {
			log.Errorf("encode error:", err)
		}

		str := base64.StdEncoding.EncodeToString(data.Bytes())

		payload, _ := json.Marshal(struct {
			Time    int64
			Payload string
		}{
			Time:    time.Now().Unix(),
			Payload: str,
		})

		cli.Publish(&client.PublishOptions{
			QoS:       mqtt.QoS0,
			TopicName: []byte("$device/stats"),
			Message:   payload,
		})
	})

	defer pub.Stop()

	p := newPublisher(inmem)

	ticker := time.NewTicker(time.Second * 5)

	go func() {

		for t := range ticker.C {
			log.Infof("Tick at %v", t)
			p.flush()
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
