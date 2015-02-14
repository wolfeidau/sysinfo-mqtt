package main

import (
	"encoding/json"
	"time"

	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"github.com/alecthomas/kingpin"
	"github.com/juju/loggo"
)

var (
	debug    = kingpin.Flag("debug", "Enable debug mode.").OverrideDefaultFromEnvar("DEBUG").Bool()
	daemon   = kingpin.Flag("daemon", "Run in daemon mode.").Short('d').Bool()
	mqttURL  = kingpin.Flag("mqttUrl", "The MQTT url to publish too.").Short('u').Default("tcp://localhost:1883").String()
	interval = kingpin.Flag("interval", "Publish interval.").Short('i').Default("5").Int()

	log    = loggo.GetLogger("sysinfo_mqtt")
	client *MQTT.MqttClient
)

func main() {
	kingpin.Version(Version)
	kingpin.Parse()

	// apply flags
	if *debug {
		loggo.GetLogger("").SetLogLevel(loggo.DEBUG)
	} else {
		loggo.GetLogger("").SetLogLevel(loggo.INFO)
	}

	opts := MQTT.NewClientOptions().AddBroker(*mqttURL).SetClientId("sysinfo_mqtt")

	client = MQTT.NewClient(opts)

	_, err := client.Start()

	if err != nil {
		panic(err)
	}

	publisher := newPublisher("$system", publish)

	c := time.Tick(time.Duration(*interval) * time.Second)

	for _ = range c {
		if err := publisher.flush(); err != nil {
			log.Errorf("woops %s", err)
		}
	}

}

func publish(topic string, data map[string]interface{}) error {
	payload, err := json.Marshal(data)

	if err != nil {
		panic(err)
	}

	log.Infof("topic: %s connected: %v", topic, client.IsConnected())

	receipt := client.Publish(MQTT.QOS_ZERO, topic, payload)
	<-receipt
	return nil
}
