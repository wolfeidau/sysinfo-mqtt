package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wolfeidau/gosigar"
)

type publisher struct {
	topicPrefix string
	pubFunc     publishFunc
	cpuPrev     sigar.Cpu
}

type publishFunc func(topic string, payload map[string]interface{}) error

func newPublisher(topicPrefix string, pubFunc publishFunc) *publisher {
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		log.Errorf("error reading initial cpu usage %s", err)
	}
	return &publisher{topicPrefix, pubFunc, cpu}
}

func (p *publisher) publishCPUTotals() error {

	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return err
	}

	res := make(map[string]interface{})

	delta := cpu.Delta(p.cpuPrev)

	res["user"] = delta.User
	res["nice"] = delta.Nice
	res["sys"] = delta.Sys
	res["idle"] = delta.Idle
	res["wait"] = delta.Wait
	res["total"] = delta.Total()

	p.cpuPrev = cpu

	res["usage"] = percentage(delta)
	log.Infof("cpu %f", percentage(delta))

	return p.pubFunc(p.topicPrefix+"/cpu/total", res)
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

	res := make(map[string]interface{})
	// "free", "used", "actualfree", "actualused", "total"
	res["free"] = mem.Free
	res["used"] = mem.Used
	res["actualfree"] = mem.ActualFree
	res["actualused"] = mem.ActualUsed
	res["total"] = mem.Total

	return p.pubFunc(p.topicPrefix+"/memory", res)
}

func (p *publisher) publishSwap() error {

	swap := sigar.Swap{}
	if err := swap.Get(); err != nil {
		return err
	}

	res := make(map[string]interface{})
	// "free", "used", "total"
	res["free"] = swap.Free
	res["used"] = swap.Used
	res["total"] = swap.Total

	return p.pubFunc(p.topicPrefix+"/swap", res)
}

func (p *publisher) publishUptime() error {

	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		return err
	}

	res := make(map[string]interface{})

	// "length"
	res["length"] = uptime.Length

	return p.pubFunc(p.topicPrefix+"/uptime", res)
}

func (p *publisher) publishNetworkInterfaces() error {

	fi, err := os.Open("/proc/net/dev")
	if err != nil {
		return err
	}
	defer fi.Close()

	res := make(map[string]interface{})

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

		ires := make(map[string]interface{})

		for i := 0; i < len(keys)-1; i++ {
			if v, err := strconv.Atoi(tmp[i]); err == nil {
				ires[keys[i]] = v
			} else {
				ires[keys[i]] = 0
			}
		}

		res[iface] = ires
	}

	return p.pubFunc(p.topicPrefix+"/network/interfaces", res)
}

func (p *publisher) publishDisks() error {

	fi, err := os.Open("/proc/diskstats")
	if err != nil {
		return err
	}
	defer fi.Close()

	res := make(map[string]interface{})

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

		ires := make(map[string]interface{})

		for i := 0; i < len(keys)-1; i++ {
			if v, err := strconv.Atoi(tmp[3+i]); err == nil {
				ires[keys[i]] = v
			} else {
				ires[keys[i]] = 0
			}
		}
		res[drive] = ires
	}

	return p.pubFunc(p.topicPrefix+"/diskstats", res)
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
