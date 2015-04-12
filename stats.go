package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rcrowley/go-metrics"
	"github.com/wolfeidau/gosigar"
)

type publisher struct {
	sink    metrics.Registry
	cpuPrev sigar.Cpu // keep the intial state for this metric
}

func newPublisher(sink metrics.Registry) *publisher {
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		log.Errorf("error reading initial cpu usage %s", err)
	}
	return &publisher{sink, cpu}
}

func (p *publisher) publishCPUTotals() error {

	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return err
	}

	delta := cpu.Delta(p.cpuPrev)

	p.SetGauge("cpu.totals.user", delta.User)
	p.SetGauge("cpu.totals.nice", delta.Nice)
	p.SetGauge("cpu.totals.sys", delta.Sys)
	p.SetGauge("cpu.totals.idle", delta.Idle)
	p.SetGauge("cpu.totals.wait", delta.Wait)
	p.SetGauge("cpu.totals.total", delta.Total())

	p.cpuPrev = cpu

	p.SetGaugeFloat64("cpu.totals.usage", percentage(delta))

	return nil
}

func percentage(current sigar.Cpu) float64 {

	idle := current.Wait + current.Idle

	return float64(current.Total()-idle) / float64(current.Total()) * 100
}

func (p *publisher) publishMemory() error {

	mem := sigar.Mem{}
	if err := mem.Get(); err != nil {
		return err
	}

	// "free", "used", "actualfree", "actualused", "total"
	p.SetGauge("memory.free", mem.Free)
	p.SetGauge("memory.used", mem.Used)
	p.SetGauge("memory.actualfree", mem.ActualFree)
	p.SetGauge("memory.actualused", mem.ActualUsed)
	p.SetGauge("memory.total", mem.Total)

	return nil
}

func (p *publisher) publishSwap() error {

	swap := sigar.Swap{}
	if err := swap.Get(); err != nil {
		return err
	}

	// "free", "used", "total"
	p.SetGauge("swap.free", swap.Free)
	p.SetGauge("swap.used", swap.Used)
	p.SetGauge("swap.total", swap.Total)

	return nil
}

func (p *publisher) publishUptime() error {

	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		return err
	}

	// "length"
	p.SetGaugeFloat64("uptime.length", uptime.Length)

	return nil
}

func (p *publisher) publishNetworkInterfaces() error {

	fi, err := os.Open("/proc/net/dev")
	if err != nil {
		return err
	}
	defer fi.Close()

	keys := []string{"iface",
		"recv_bytes", "recv_packets", "recv_errs",
		"recv_drop", "recv_fifo", "recv_frame",
		"recv_compressed", "recv_multicast",
		"trans_bytes", "trans_packets", "trans_errs",
		"trans_drop", "trans_fifo", "trans_colls",
		"trans_carrier", "trans_compressed"}

	// Search interface
	skip := 2
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		// Skip headers
		if skip > 0 {
			skip--
			continue
		}

		line := scanner.Text()
		tmp := strings.Split(line, ":")
		if len(tmp) < 2 {
			return fmt.Errorf("Unable to parse /proc/net/dev")
		}

		iface := strings.Trim(tmp[0], " ")
		tmp = strings.Fields(tmp[1])

		for i := 0; i < len(keys)-1; i++ {
			if v, err := strconv.Atoi(tmp[i]); err == nil {
				p.SetGauge(fmt.Sprintf("network.interfaces.%s.%s", iface, keys[i]), uint64(v))
			} else {
				p.SetGauge(fmt.Sprintf("network.interfaces.%s.%s", iface, keys[i]), 0)
			}
		}
	}

	return nil
}

func (p *publisher) publishDisks() error {

	fi, err := os.Open("/proc/diskstats")
	if err != nil {
		return err
	}
	defer fi.Close()

	keys := []string{"device",
		"read_ios", "read_merges", "read_sectors", "read_ticks",
		"write_ios", "write_merges", "write_sectors", "write_ticks",
		"in_flight", "io_ticks", "time_in_queue"}

	// Search device
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		tmp := strings.Fields(scanner.Text())
		if len(tmp) < 14 {
			return fmt.Errorf("Unable to parse /proc/diskstats")
		}

		drive := tmp[2]

		for i := 0; i < len(keys)-1; i++ {
			if v, err := strconv.Atoi(tmp[3+i]); err == nil {
				p.SetGauge(fmt.Sprintf("diskstats.%s.%s", drive, keys[i]), uint64(v))
			} else {
				p.SetGauge(fmt.Sprintf("diskstats.%s.%s", drive, keys[i]), 0)
			}
		}
	}

	return nil
}

func (p *publisher) SetGaugeFloat64(key string, val float64) {
	metrics.GetOrRegisterGaugeFloat64(key, p.sink).Update(val)
}

func (p *publisher) SetGauge(key string, val uint64) {
	metrics.GetOrRegisterGauge(key, p.sink).Update(int64(val))
}

func (p *publisher) flush() error {

	if err := p.publishCPUTotals(); err != nil {
		return err
	}

	if err := p.publishMemory(); err != nil {
		return err
	}

	if err := p.publishSwap(); err != nil {
		return err
	}

	if err := p.publishUptime(); err != nil {
		return err
	}

	if err := p.publishNetworkInterfaces(); err != nil {
		return err
	}

	if err := p.publishDisks(); err != nil {
		return err
	}

	return nil
}

func (p *publisher) export() map[string]interface{} {
	return exportMetrics(p.sink)
}
