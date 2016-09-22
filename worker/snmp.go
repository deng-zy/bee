package worker

import (
	"encoding/json"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/soniah/gosnmp"
	"github.com/tidwall/gjson"
	"os"
	"time"
)

const (
	DEFAULT_INTERVAL = 5
	DEFULAT_PORT     = 161
)

type watchResult struct {
	Host    string `json:"host"`
	IsAlive bool   `json:"isAlive"`
}

type SNMP struct {
	port     uint16
	running  bool
	routines uint16
	oid      []string
	ch       chan watchResult
	setting  *ini.Section
	hosts    []gjson.Result
	interval time.Duration
}

func NewSNMP(setting *ini.Section, hosts gjson.Result) (*SNMP, error) {
	if hosts.String() == "" {
		panic(fmt.Errorf("watch snmp host list empty. origin:%s", hosts.String()))
	}

	return &SNMP{
		port:     DEFULAT_PORT,
		running:  true,
		routines: 0,
		hosts:    hosts.Array(),
		ch:       make(chan watchResult),
		setting:  setting,
		interval: 3 * time.Second,
		oid:      []string{"1.3.6.1.2.1.1.5.0"},
	}, nil
}

func (s *SNMP) Port(port uint16) {
	s.port = port
}

func (s *SNMP) Boot() {
	for _, host := range s.hosts {
		interval, err := time.ParseDuration(host.Get("interval").String())
		if err != nil {
			interval = DEFAULT_INTERVAL
		}
		go s.watch(host.Get("host").String(), interval)
		s.routines++
	}
	s.receiver()
	fmt.Fprintln(os.Stdout, "snmp worker shutdown")
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
	client = s.createClient(host)
	interval = interval * time.Second

	for s.running {
		err := client.Connect()
		alive := false
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %v %v", host, s.oid, err)
			s.send(host, false)
			time.Sleep(interval)
			continue
		} else {
			result, err := client.Get(s.oid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "get error.%s %s %s\n", host, s.oid[0], err)
			} else {
				for _, v := range result.Variables {
					switch v.Type {
					case gosnmp.OctetString:
						fmt.Fprintf(os.Stdout, "%s %s %s\n", host, s.oid[0], v.Value.([]byte))
					}
				}
				alive = true
			}
			s.send(host, alive)
			client.Conn.Close()
			time.Sleep(interval)
		}
	}
	s.routines--
}

func (s *SNMP) send(host string, isAlive bool) {
	go func() {
		s.ch <- watchResult{Host: host, IsAlive: isAlive}
	}()
}

func (s *SNMP) receiver() {
	for s.running || s.routines < 1 {
		result := make([]watchResult, len(s.hosts))
		for index, _ := range s.hosts {
			result[index] = <-s.ch
		}

		origin, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Printf("%s\n", origin)
		}
		time.Sleep(s.interval)
	}
}
