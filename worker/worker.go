package worker

import (
	"fmt"
	u "net/url"
)

type workerStruct interface {
	Boot()
	Stop()
}

type Packet struct {
	name string
	data []byte
	ts   int64
}

func combineQuery(rawurl string, params string) (string, error) {
	base, err := u.Parse(rawurl)
	if err != nil {
		return "", fmt.Errorf("parse url: %s error. error:%s", params, err)
	}

	base.RawQuery = fmt.Sprintf("%s%s", base.RawQuery, params)
	return base.String(), nil
}
