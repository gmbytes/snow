package configuration

import (
	"sync"

	"github.com/gmbytes/snow/pkg/notifier"
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
			defer func() { _ = recover() }()
			callback()
		}()
	}
}
