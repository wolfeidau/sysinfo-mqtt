package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rcrowley/go-metrics"
	"golang.org/x/net/websocket"
)

var pingPeriod = 5 * time.Second

type wsServer struct {
	localRegistry metrics.Registry
}

type wsMessage struct {
	Timestamp int64                  `json:"ts"`
	Data      map[string]interface{} `json:"payload"`
}

func newWsServer(localRegistry metrics.Registry) *wsServer {
	return &wsServer{localRegistry}
}

func (w *wsServer) wsPublisher(ws *websocket.Conn) {

	pingTicker := time.NewTicker(pingPeriod)

	defer func() {
		pingTicker.Stop()
		ws.Close()
	}()

	for {

		payload := exportMetrics(w.localRegistry)

		data, _ := json.Marshal(wsMessage{time.Now().Unix(), payload})

		n, err := ws.Write(data)

		log.Debugf("publishing to %s length %d", ws.RemoteAddr(), n)

		if err != nil {
			log.Errorf("write failed: %s", err)
			return
		}

		// wait for the next tick
		<-pingTicker.C

	}
}

func (w *wsServer) listenAndServ(port int) {

	http.Handle("/sysmon", websocket.Handler(w.wsPublisher))

	log.Debugf("Listening for websocket requests on %d", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Errorf("listener failed: %s", err)
	}
}
