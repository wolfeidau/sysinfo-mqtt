package main

import (
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

type sysInfoEngine struct {
	murl      *url.URL
	cli       *client.Client
	cliLock   sync.Mutex
	connected bool

	publisher  *publisher
	pubTicker  *time.Ticker
	pollTicker *time.Ticker
}

func newSysInfoEngine(publisher *publisher) (*sysInfoEngine, error) {

	murl, err := url.Parse(*mqttURL)

	if err != nil {
		return nil, err
	}

	si := &sysInfoEngine{}

	// Create an MQTT Client.
	cli := client.New(&client.Options{
		ErrorHandler: si.handleClientError,
	})

	si.murl = murl
	si.cli = cli

	si.attemptConnect()

	si.publisher = publisher
	si.pollTicker = time.NewTicker(time.Second * 1)
	si.pubTicker = time.NewTicker(time.Second * 15)

	go poll(si)
	go publish(si)

	return si, nil
}

func (si *sysInfoEngine) attemptConnect() bool {

	log.Debugf("Attempt Connect")

	si.cliLock.Lock()

	defer si.cliLock.Unlock()

	if si.connected {
		log.Debugf("already connected")
		return si.connected
	}

	log.Debugf("connecting to %s", si.murl.Host)

	co := &client.ConnectOptions{
		Network:  si.murl.Scheme,
		Address:  si.murl.Host,
		ClientID: []byte("sysinfo-mqtt"),
	}

	if si.murl.User != nil {
		co.UserName = []byte(si.murl.User.Username())
		if pass, ok := si.murl.User.Password(); ok {
			co.Password = []byte(pass)
		}
	}

	// Connect to the MQTT Server.
	if err := si.cli.Connect(co); err != nil {
		log.Errorf("failed to connect: %s", err)
		si.connected = false
	} else {
		si.connected = true
	}

	return si.connected
}

func (si *sysInfoEngine) handleClientError(err error) {

	log.Errorf("client error: %s", err)

	si.cliLock.Lock()
	defer si.cliLock.Unlock()

	si.connected = false

	go func() {
		if err := si.cli.Disconnect(); err != nil {
			log.Errorf("client disconnect error: %s", err)
		}
		log.Debugf("client disconnected")
	}()
}

func (si *sysInfoEngine) disconnect() error {
	if si.connected {
		return si.cli.Disconnect()
	}
	return nil
}

func publish(si *sysInfoEngine) {

	for t := range si.pubTicker.C {

		metrics := si.publisher.export()

		if !si.attemptConnect() {
			log.Warningf("publish failed: not connected")
		}

		payload, _ := json.Marshal(struct {
			Time    int64                  `json:"ts"`
			Payload map[string]interface{} `json:"payload"`
		}{
			Time:    t.Unix(),
			Payload: metrics,
		})

		log.Debugf("publishing to %s length %d", statsTopic, len(payload))

		err := si.cli.Publish(&client.PublishOptions{
			QoS:       mqtt.QoS0,
			TopicName: []byte(statsTopic),
			Message:   payload,
		})

		if err != nil {
			log.Errorf("error publishing: %s", err)
		}
	}

}

func poll(si *sysInfoEngine) {

	for _ = range si.pollTicker.C {
		//log.Debugf("Flush at %v", t)
		si.publisher.flush()
	}

}
