package task

import (
	"fmt"

	"github.com/panjf2000/ants/v2"
)

const goroutinePoolSize = 100000

var p *ants.PoolWithFunc

//nolint:gochecknoinits // goroutine pool must be initialized at package load time
func init() {
	var err error
	p, err = ants.NewPoolWithFunc(goroutinePoolSize, func(f any) {
		(f.(func()))()
	}, ants.WithPreAlloc(true))

	if err != nil {
		panic(fmt.Sprintf("init goroutine pool: %v", err))
	}
}

func Execute(f func()) {
	_ = p.Invoke(f)
}
