package configuration

import (
	"sync"

	"github.com/mogud/snow/core/notifier"
)

var _ notifier.INotifier = (*Notifier)(nil)

type Notifier struct {
	lock      sync.Mutex
	callbacks []func()
}

func NewNotifier() *Notifier {
	return &Notifier{}
}

func (ss *Notifier) RegisterNotifyCallback(callback func()) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	ss.callbacks = append(ss.callbacks, callback)
}

func (ss *Notifier) Notify() {
	var cbs []func()
	ss.lock.Lock()
	cbs = ss.callbacks
	ss.lock.Unlock()

	for _, callback := range cbs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 防止一个 callback 的 panic 影响其他 callback
					// 这里可以记录日志，但为了避免循环依赖，暂时不记录
				}
			}()
			callback()
		}()
	}
}
