package configuration

import (
	"sort"
	"strings"
	"sync"

	"github.com/mogud/snow/core/notifier"
)

var _ IConfigurationProvider = (*Provider)(nil)

type Provider struct {
	lock     sync.Mutex
	data     *CaseInsensitiveStringMap[string]
	notifier *Notifier
}

func NewProvider() *Provider {
	return &Provider{
		data:     NewCaseInsensitiveStringMap[string](),
		notifier: NewNotifier(),
	}
}

func (ss *Provider) Get(key string) string {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	return ss.data.Get(key)
}

func (ss *Provider) TryGet(key string) (value string, ok bool) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	value, ok = ss.data.TryGet(key)
	return
}

func (ss *Provider) Set(key string, value string) {
	ss.lock.Lock()
	defer ss.lock.Unlock()
	ss.data.Add(key, value)
}

func (ss *Provider) GetReloadNotifier() notifier.INotifier {
	return ss.notifier
}

func (ss *Provider) Replace(data *CaseInsensitiveStringMap[string]) {
	ss.lock.Lock()
	ss.data = data
	ss.lock.Unlock()

	// 通知监听者数据已更新
	ss.OnReload()
}

func (ss *Provider) Load() {
}

func (ss *Provider) OnReload() {
	ss.notifier.Notify()
}

func (ss *Provider) GetChildKeys(parentPath string) []string {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	return getSortedSegmentChildKeys(ss.data, parentPath)
}

func getSortedSegmentChildKeys(m *CaseInsensitiveStringMap[string], parentPath string) []string {
	childKeys := make([]string, 0)
	if len(parentPath) == 0 {
		m.Scan(func(key, _ string) {
			childKeys = append(childKeys, keySegment(key, 0))
		})
	} else {
		upperParentPath := strings.ToUpper(parentPath)
		m.ScanFull(func(upperKey, key, _ string) {
			if len(upperKey) > len(parentPath) && strings.HasPrefix(upperKey, upperParentPath) && upperKey[len(parentPath)] == ':' {
				childKeys = append(childKeys, keySegment(key, len(parentPath)+1))
			}
		})
	}
	sort.Strings(childKeys)
	return childKeys
}

func keySegment(key string, prefixLength int) string {
	if prefixLength >= len(key) {
		return ""
	}
	idx := strings.IndexByte(key[prefixLength:], ':')
	if idx == -1 {
		return key[prefixLength:]
	}

	return key[prefixLength : prefixLength+idx]
}
