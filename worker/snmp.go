package worker

import (
	"encoding/json"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/soniah/gosnmp"
	"github.com/tidwall/gjson"
	"os"
	"sync"
	"time"
)

const (
	DEFAULT_INTERVAL = 5
	DEFAULT_PORT     = 161
)

type watchResult struct {
	Host    string `json:"host"`
	IsAlive bool   `json:"isAlive"`
}

type SNMP struct {
	port     uint16
	running  bool
	oid      []string
	ch       chan watchResult
	setting  *ini.Section
	hosts    []gjson.Result
	interval time.Duration
}

func NewSNMP(setting *ini.Section, hosts gjson.Result) *SNMP {
	if hosts.String() == "" {
		panic(fmt.Errorf("watch snmp host list empty. origin:%s", hosts.String()))
	}

	return &SNMP{
		port:     DEFAULT_PORT,
		running:  true,
		hosts:    hosts.Array(),
		ch:       make(chan watchResult, 1024),
		setting:  setting,
		interval: 3 * time.Second,
		oid:      []string{"1.3.6.1.2.1.1.5.0"},
	}
}

func (s *SNMP) Port(port uint16) {
	s.port = port
}

func (s *SNMP) Boot(wg sync.WaitGroup) {
	for _, host := range s.hosts {
		interval, err := time.ParseDuration(host.Get("interval").String())
		if err != nil {
			interval = DEFAULT_INTERVAL
		}
		go s.watch(host.Get("host").String(), interval * time.Second)
	}
	s.receiver()
	fmt.Fprintln(os.Stdout, "snmp worker shutdown")
	wg.Done()
}

func (s *SNMP) Stop() {
	s.running = false
}

func (s *SNMP) createClient(host string) *gosnmp.GoSNMP {
	timeout, err := time.ParseDuration(s.setting.Key("timeout").String())
	userName := s.setting.Key("username").String()
	password := s.setting.Key("password").String()

	if err != nil || timeout == 0 {
		timeout = 1
	}

	return &gosnmp.GoSNMP{
		Target:        host,
		Port:          s.port,
		Version:       gosnmp.Version3,
		Timeout:       timeout * time.Second,
		Retries:       0,
		SecurityModel: gosnmp.UserSecurityModel,
		MsgFlags:      gosnmp.AuthNoPriv,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 userName,
			AuthenticationProtocol:   gosnmp.MD5,
			AuthenticationPassphrase: password,
		},
	}
}

func (s *SNMP) watch(host string, interval time.Duration) {
	var client *gosnmp.GoSNMP
	var connErr error

	client = s.createClient(host)
	connErr = client.Connect()

	for s.running {
		var alive bool = true

		if connErr != nil {
			fmt.Fprintf(os.Stderr, "connection to %s fail. error: %s\n", host, connErr)
			alive = false
		} else {
			_, err := client.Get(s.oid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "host:%s get %s fail. error:%s\n", host, s.oid[0], err)
				alive = false
			}
		}

		s.send(host, alive)
		time.Sleep(interval)
	}
}

func (s *SNMP) send(host string, isAlive bool) {
	go func() {
		s.ch <- watchResult{Host: host, IsAlive: isAlive}
	}()
}

func (s *SNMP) receiver() {
	for s.running || len(s.ch) > 0 {
		if len(s.ch) < 1 {
			time.Sleep(s.interval)
			continue
		}

		result := make([]watchResult, len(s.hosts))
		for i := range s.hosts {
			result[i] = <-s.ch
		}

		origin, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", origin)
		}
	}
}
