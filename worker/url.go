package worker

import (
	"encoding/json"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

const URL_DEFAULT_TIMEOUT = 5
const URL_DEFAULT_INTERVAL = 5

type urlResult struct {
	Url     string `json:"url"`
	Status  int    `json:"status"`
	Body    string `json:"body"`
	Message string `json:"message"`
}

type Url struct {
	urls     []gjson.Result
	urlsLen  int
	client   *http.Client
	timeout  time.Duration
	interval time.Duration
	running  bool
	ch       chan urlResult
}

func NewUrl(setting *ini.Section, urls gjson.Result) *Url {
	if urls.String() == "" {
		panic("url list empty")
	}

	urlArray := urls.Array()
	urlArrayLen := len(urlArray)
	if urlArrayLen == 0 {
		panic("urls[] length 0")
	}

	timeout, err := time.ParseDuration(setting.Key("timeout").String())
	if err != nil {
		timeout = URL_DEFAULT_TIMEOUT * time.Second
	}

	interval, err := time.ParseDuration(setting.Key("interval").String())
	if err != nil {
		interval = URL_DEFAULT_INTERVAL * time.Second
	} else {
		interval = interval * time.Second
	}

	return &Url{
		urls:     urlArray,
		urlsLen:  urlArrayLen,
		timeout:  timeout,
		interval: interval,
		running:  true,
		ch:       make(chan urlResult, urlArrayLen),
	}
}

func (s *Url) Boot(wg sync.WaitGroup) {
	for _, url := range s.urls {
		interval, err := time.ParseDuration(url.Get("interval").String())
		if err != nil {
			interval = s.interval
		} else {
			interval = interval * time.Second
		}

		go s.watch(url.Get("url").String(), interval, url.Get("param").String())
	}

	s.receiver()
	fmt.Fprint(os.Stdout, "url worker shutdown")
	wg.Done()
}

func (s *Url) SetTimeout(timeout time.Duration) {
	s.timeout = timeout * time.Second
}

func (s *Url) SetInterval(interval time.Duration) {
	s.interval = interval * time.Second
}

func (s *Url) Stop() {
	s.running = false
}

func (s *Url) watch(watchUrl string, interval time.Duration, param string) {
	s.setClient()

	if param != "" {
		var err error
		watchUrl, err = combineQuery(watchUrl, param)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}

	for s.running {
		response, err := s.client.Get(watchUrl)
		if err != nil {
			s.send(watchUrl, 0, "", fmt.Sprintf("%s", err))
		} else {
			if s.isOk(response) {
				body, err := ioutil.ReadAll(response.Body)
				if err != nil {
					s.send(watchUrl, response.StatusCode, "", "")
				} else {
					s.send(watchUrl, response.StatusCode, string(body), "")
				}
			} else {
				s.send(watchUrl, response.StatusCode, "", response.Status)
			}
		}
		time.Sleep(interval)
	}
}

func (s *Url) setClient() {
	s.client = &http.Client{
		Timeout: s.timeout,
	}
}

func (s *Url) receiver() {
	for s.running || len(s.ch) > 0 {
		result := make([]urlResult, s.urlsLen)
		if len(s.ch) < 1 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		for i := range s.urls {
			result[i] = <-s.ch
		}
		jsonStr, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "json encode error. %v", err)
		} else {
			fmt.Fprintf(os.Stdout, "%s\n", jsonStr)
		}
	}

	close(s.ch)
}

func (s *Url) send(url string, status int, body string, msg string) {
	go func() {
		s.ch <- urlResult{Url: url, Status: status, Body: body, Message: msg}
	}()
}

func (s *Url) isOk(response *http.Response) bool {
	if http.StatusOK == response.StatusCode {
		return true
	}
	return false
}
