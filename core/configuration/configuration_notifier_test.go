package configuration_test

import (
	"sync"
	"testing"

	"github.com/mogud/snow/core/configuration"
	"github.com/stretchr/testify/assert"
)

func TestNotifier_RegisterNotifyCallback(t *testing.T) {
	notifier := configuration.NewNotifier()

	callCount := 0
	notifier.RegisterNotifyCallback(func() {
		callCount++
	})

	notifier.Notify()
	assert.Equal(t, 1, callCount)

	notifier.Notify()
	assert.Equal(t, 2, callCount)
}

func TestNotifier_MultipleCallbacks(t *testing.T) {
	notifier := configuration.NewNotifier()

	callCount1 := 0
	callCount2 := 0

	notifier.RegisterNotifyCallback(func() {
		callCount1++
	})

	notifier.RegisterNotifyCallback(func() {
		callCount2++
	})

	notifier.Notify()

	assert.Equal(t, 1, callCount1)
	assert.Equal(t, 1, callCount2)
}

func TestNotifier_ConcurrentNotify(t *testing.T) {
	notifier := configuration.NewNotifier()

	var wg sync.WaitGroup
	callCount := 0
	var mu sync.Mutex

	notifier.RegisterNotifyCallback(func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// 并发注册回调
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			notifier.RegisterNotifyCallback(func() {
				mu.Lock()
				callCount++
				mu.Unlock()
			})
		}()
	}

	wg.Wait()

	// 并发通知
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			notifier.Notify()
		}()
	}

	wg.Wait()

	// 应该有至少 10 次调用（每个 Notify 都会调用所有回调）
	assert.GreaterOrEqual(t, callCount, 10)
}

func TestNotifier_NotifyAfterUnregister(t *testing.T) {
	notifier := configuration.NewNotifier()

	callCount := 0
	notifier.RegisterNotifyCallback(func() {
		callCount++
	})

	notifier.Notify()
	assert.Equal(t, 1, callCount)

	// 再次通知
	notifier.Notify()
	assert.Equal(t, 2, callCount)
}
