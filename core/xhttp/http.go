package xhttp

import (
	"crypto/tls"
	"net/http"
	"time"
)

type Transport = http.Transport

type HttpClient = http.Client

func Client() *HttpClient {
	// TODO  Golang HTTP2 有 bug，会导致超时访问，使用 HTTP1 可以绕过
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"h1"},
		},
	}
	return &http.Client{
		Timeout:   time.Second * 5,
		Transport: tr,
	}
}
