package main

import (
	"sync"
	"time"

	"github.com/gmbytes/snow/routines/node"
)

type ping struct {
	node.Service

	closeChan chan struct{}
	pongProxy node.IProxy
}

func (ss *ping) Start(_ any) {
	ss.Infof("ping start")
	ss.closeChan = make(chan struct{})

	// CreateProxy 内部会优先通过 ServiceDiscovery.Resolve 查找 "Pong" 的地址，
	// 失败时回退到静态配置表。对调用方完全透明。
	ss.pongProxy = ss.CreateProxy("Pong")

	go func() {
		ticker := time.NewTicker(3 * time.Second)
	loop:
		for {
			select {
			case <-ticker.C:
				ss.Fork("rpc", func() {
					ss.pongProxy.Call("Hello", "ping via discovery").
						Then(func(ret string) {
							ss.Infof("received: %s", ret)
						}).Done()
				})
			case <-ss.closeChan:
				break loop
			}
		}
	}()
}

func (ss *ping) Stop(_ *sync.WaitGroup) {
	ss.Infof("ping stop")
	close(ss.closeChan)
}

func (ss *ping) AfterStop() {}
