package main

import (
	"compress/zlib"
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"strings"
	"bytes"
	"io/ioutil"
)

func formatParams(params map[string]string) string {
	argsNum := len(params)
	keys := make([]string, argsNum)
	args := make([]string, argsNum)
	index := 0

	for k := range params {
		keys[index] = k
		index++
	}
	sort.Strings(keys)

	for i, k := range keys {
		args[i] = fmt.Sprintf("%s=%s", k, params[k])
	}

	return strings.Join(args, "^_^")

}

func hashSign(origin string) string {
	md5Hash := md5.New()
	io.WriteString(md5Hash, origin)
	return fmt.Sprintf("%x", md5Hash.Sum(nil))
}

func compress(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	z, err := zlib.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	p, err := ioutil.ReadAll(z)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func generatorSign(params map[string]string, data []byte, secret string) string {
	var source []string = []string{formatParams(params), string(data), secret}
	return hashSign(strings.Join(source, "^_^"))
}
