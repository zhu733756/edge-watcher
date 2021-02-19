// Copyright 2020 The KubeSphere Authors. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-iptables/iptables"
	"github.com/fsnotify/fsnotify"
)

type IPTablesEntry struct {
	Table           string `json:"table"`
	Chain           string `json:"chain"`
	Jump            string `json:"jump"`
	Protocol        string `json:"protocol"`
	SourceIP        string `json:"source_ip"`
	SourcePort      string `json:"source_port"`
	DestinationIP   string `json:"destination_ip"`
	DestinationPort string `json:"destination_port"`
}

type IPTablesConf struct {
	IPTables []IPTablesEntry `json:"iptables"`
}

func exitHandler(signalCh chan string) {
	c := make(chan os.Signal)

	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				ExitFunc(signalCh)
			case syscall.SIGUSR1:
			case syscall.SIGUSR2:
			default:
			}
		}
	}()
}

func ExitFunc(signalCh chan string) {
	log.Println("Exiting...")
	signalCh <- "exit"
}

func existsIPTable(ipt *iptables.IPTables, table, chain, jump, protocol, sourceIP, sourcePort, destinationIP, destinationPort string) bool {
	exists, err := ipt.Exists(table, chain, "-p", protocol, "--dport", sourcePort, "-d", sourceIP, "-j", jump, "--to-destination", destinationIP+":"+destinationPort)
	if err != nil {
		log.Printf("Exists failed: %v\n", err)
		return false
	} else {
		return exists
	}
}

func ensureIPTables(iptablesConf *IPTablesConf) {
	if iptablesConf == nil {
		log.Println("iptables empty")
		return
	}

	ipt, err := iptables.New()
	if err != nil {
		log.Printf("Create iptables error: %+v\n", err)
		return
	}

	for _, iptablesEntry := range iptablesConf.IPTables {
		if !existsIPTable(ipt, iptablesEntry.Table, iptablesEntry.Chain, iptablesEntry.Jump, iptablesEntry.Protocol, iptablesEntry.SourceIP, iptablesEntry.SourcePort, iptablesEntry.DestinationIP, iptablesEntry.DestinationPort) {
			err = ipt.Append(iptablesEntry.Table, iptablesEntry.Chain, "-p", iptablesEntry.Protocol, "--dport", iptablesEntry.SourcePort, "-d", iptablesEntry.SourceIP, "-j", iptablesEntry.Jump, "--to-destination", iptablesEntry.DestinationIP+":"+iptablesEntry.DestinationPort)
			if err != nil {
				log.Printf("Append failed: %v\n", err)
			}
		}
	}
}

func clearIPTables(iptablesConf *IPTablesConf, maxDuplicateCount int) {
	if iptablesConf == nil {
		log.Println("iptables empty")
		return
	}

	ipt, err := iptables.New()
	if err != nil {
		log.Printf("Create iptables error: %+v\n", err)
		return
	}

	for _, iptablesEntry := range iptablesConf.IPTables {
		for i := 0; i < maxDuplicateCount; i++ {
			if existsIPTable(ipt, iptablesEntry.Table, iptablesEntry.Chain, iptablesEntry.Jump, iptablesEntry.Protocol, iptablesEntry.SourceIP, iptablesEntry.SourcePort, iptablesEntry.DestinationIP, iptablesEntry.DestinationPort) {
				err = ipt.Delete(iptablesEntry.Table, iptablesEntry.Chain, "-p", iptablesEntry.Protocol, "--dport", iptablesEntry.SourcePort, "-d", iptablesEntry.SourceIP, "-j", iptablesEntry.Jump, "--to-destination", iptablesEntry.DestinationIP+":"+iptablesEntry.DestinationPort)
				if err != nil {
					log.Printf("Delete failed: %v\n", err)
				}
			} else {
				break
			}
		}
	}
}

func loadIPTables(fileName string, iptablesConf *IPTablesConf) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Printf("Read iptables.conf error: %v\n", err)
		return
	}
	err = json.Unmarshal(data, iptablesConf)
	if err != nil {
		log.Printf("Read iptables.conf error: %v\n", err)
		return
	}
}

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("File watcher NewWatcher failed: %+v\n", err)
		return
	}
	defer watcher.Close()

	signalCh := make(chan string, 10)
	exitHandler(signalCh)

	err = watcher.Add("/etc/iptables.conf/iptables.conf")
	if err != nil {
		log.Printf("File watcher watch config file failed: %+v\n", err)
		return
	}

	iptablesConf := IPTablesConf{}

	loadIPTables("/etc/iptables.conf/iptables.conf", &iptablesConf)
	ensureIPTables(&iptablesConf)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("File watcher event not ok")
				continue
			}
			if event.Op == fsnotify.Remove {
				watcher.Remove(event.Name)
				watcher.Add("/etc/iptables.conf/iptables.conf")

				log.Println("IPTables file changed")
				clearIPTables(&iptablesConf, 10)
				loadIPTables("/etc/iptables.conf/iptables.conf", &iptablesConf)
				ensureIPTables(&iptablesConf)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Printf("File watcher error: %+v\n", err)
				os.Exit(0)
				return
			}
		case <-signalCh:
			log.Println("Received exit signal...")
			//Drain SignalCh
			for len(signalCh) > 0 {
				<-signalCh
			}
			clearIPTables(&iptablesConf, 10)
			os.Exit(0)
			return
		}
	}
}
