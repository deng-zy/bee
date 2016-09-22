package main

import (
	"errors"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func fetchAgentSetting(apiSetting *ini.Section) (gjson.Result) {
	url, err := getAgentSettingUrl(apiSetting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(-5)
	}

	settings, err := fetchContent(url, 1)
	if len(settings) == 0 || err != nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "%s response body empty\n", err)
		}
		os.Exit(-5)
	}
	return gjson.GetBytes(settings, "data")
}

func getAgentSettingUrl(setting *ini.Section) (string, error) {
	url := setting.Key("setting").String()
	if url == "" {
		return "", errors.New("unknown agent setting url")
	}
	return url, nil
}

func fetchContent(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{
		Timeout: timeout * time.Second,
	}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("%s response error. StatusCode:%v", url, response.StatusCode)
	}

	contentType := response.Header.Get("content-type")
	if !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("%s content-type:%s", url, contentType)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading error. url:%s, error:%v", url, err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("%s response body length:%d", url, len(body))
	}

	status := gjson.GetBytes(body, "status")
	data := gjson.GetBytes(body, "data")
	if status.Int() != 0 || data.String() == "null" {
		return nil, fmt.Errorf("%s api response error. status:%d, body:%s", url, status.Int(), data.String())
	}

	return body, nil
}
