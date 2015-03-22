package main

import (
	"sync"
	"time"

	"github.com/armon/go-metrics"
)

type InmemPublishFunc func(metrics map[string]interface{})

// InmemPublish is used to invoke a function with a snapshot of the
// current metrics based on an inverval.
type InmemPublish struct {
	publishFunc InmemPublishFunc
	inm         *metrics.InmemSink

	ticker   *time.Ticker
	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// NewInmemPublish creates a new InmemSignal which publishes stats to function.
func NewInmemPublish(inmem *metrics.InmemSink, interval time.Duration, publishFunc InmemPublishFunc) *InmemPublish {
	i := &InmemPublish{
		publishFunc: publishFunc,
		ticker:      time.NewTicker(interval),
		inm:         inmem,
		stopCh:      make(chan struct{}),
	}
	go i.run()
	return i
}

// Stop is used to stop the InmemSignal from listening
func (i *InmemPublish) Stop() {
	i.stopLock.Lock()
	defer i.stopLock.Unlock()

	if i.stop {
		return
	}
	i.stop = true
	close(i.stopCh)
}

// run is a long running routine that handles signals
func (i *InmemPublish) run() {
	for {
		select {
		case <-i.ticker.C:
			i.dumpStats()
		case <-i.stopCh:
			return
		}
	}
}

// dumpStats is used to dump the data to output writer
func (i *InmemPublish) dumpStats() {

	m := make(map[string]interface{})

	data := i.inm.Data()
	// Skip the last period which is still being aggregated
	for i := 0; i < len(data)-1; i++ {
		intv := data[i]
		intv.RLock()
		for name, val := range intv.Gauges {
			m[name] = val
		}
		for name, vals := range intv.Points {
			m[name] = vals
		}
		for name, agg := range intv.Counters {
			m[name] = agg
		}
		for name, agg := range intv.Samples {
			m[name] = agg
		}
		intv.RUnlock()
	}

	// Write out the bytes
	i.publishFunc(m)
}
