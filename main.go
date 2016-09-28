package main

import (
	"bvc_bee/worker"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/urfave/cli"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"sync"
)

func main() {
	var iniFile string
	app := cli.NewApp()
	app.Name = "cli"
	app.Version = "1.0.0"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "ini, i",
			Value: "etc/bee.ini",
			Usage: "please input bee.ini",
		},
	}

	app.Action = func(c *cli.Context) error {
		iniFile = c.String("ini")
		if !path.IsAbs(path.Dir(iniFile)) {
			dir, err := os.Getwd()
			if err != nil {
				fmt.Println("get current directory error")
				os.Exit(-1)
			}

			iniFile = path.Join(dir, iniFile)
			if _, err := os.Stat(iniFile); os.IsNotExist(err) || os.IsPermission(err) {
				fmt.Printf("%s not exists or not permission\n", iniFile)
				os.Exit(-2)
			}
		}
		return nil
	}

	app.Run(os.Args)
	bootWorkers(iniFile)
}

func bootWorkers(iniFile string) {
	exitCode := -4
	config, err := ini.InsensitiveLoad(iniFile)
	if err != nil {
		fmt.Printf("parse %s error. error: %v\n", iniFile, err)
		os.Exit(exitCode)
	}

	apiSetting, err := config.GetSection("api")
	if err != nil {
		fmt.Printf("not found api section in %s file", iniFile)
		os.Exit(exitCode)
	}

	agentSetting := fetchAgentSetting(apiSetting)
	SNMPSetting, err := config.GetSection("snmp")
	if err != nil {
		fmt.Printf("not found snmp section in %s file", iniFile)
		os.Exit(exitCode)
	}

	URLSetting, err := config.GetSection("url")
	if err != nil {
		fmt.Printf("not found url section in %s file", iniFile)
		os.Exit(exitCode)
	}

	var wg sync.WaitGroup

	var SNMPWorker *worker.SNMP
	SNMPWorker = worker.NewSNMP(SNMPSetting, agentSetting.Get("snmp"))

	var URLWorker *worker.Url
	URLWorker = worker.NewUrl(URLSetting, agentSetting.Get("url"))

	if err != nil {
		panic(err)
	}

	go func() {
		http.ListenAndServe("localhost:7777", nil)
	}()

	wg.Add(2)
	go SNMPWorker.Boot(wg)
	go URLWorker.Boot(wg)
	wg.Wait()
	os.Exit(0)
}
