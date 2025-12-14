package configuration

import (
	"sync"

	"github.com/mogud/snow/core/notifier"
)

var _ IConfigurationRoot = (*Root)(nil)

type Root struct {
	lock sync.Mutex

	providers []IConfigurationProvider
	notifier  *Notifier
}

func NewConfigurationRoot(providers []IConfigurationProvider) IConfigurationRoot {
	root := &Root{
		providers: providers,
		notifier:  NewNotifier(),
	}

	for _, provider := range providers {
		provider.GetReloadNotifier().RegisterNotifyCallback(root.notifier.Notify)
		provider.Load()
	}

	return root
}

func (ss *Root) Get(key string) string {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for i := len(ss.providers) - 1; i >= 0; i-- {
		if value, ok := ss.providers[i].TryGet(key); ok {
			return value
		}
	}
	return ""
}

func (ss *Root) TryGet(key string) (value string, ok bool) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for i := len(ss.providers) - 1; i >= 0; i-- {
		if value, ok := ss.providers[i].TryGet(key); ok {
			return value, true
		}
	}
	return "", false
}

func (ss *Root) Set(key string, value string) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for _, provider := range ss.providers {
		provider.Set(key, value)
	}
}

func (ss *Root) GetSection(key string) IConfigurationSection {
	return NewSection(ss, key)
}

func (ss *Root) GetChildren() []IConfigurationSection {
	return ss.GetChildrenByPath("")
}

func (ss *Root) GetChildrenByPath(path string) []IConfigurationSection {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	keySet := make(map[string]struct{})
	for _, provider := range ss.providers {
		for _, key := range provider.GetChildKeys(path) {
			keySet[key] = struct{}{}
		}
	}

	sections := make([]IConfigurationSection, 0, len(keySet))
	if len(path) > 0 {
		for key := range keySet {
			sections = append(sections, ss.GetSection(path+KeyDelimiter+key))
		}
	} else {
		for key := range keySet {
			sections = append(sections, ss.GetSection(key))
		}
	}
	return sections
}

func (ss *Root) GetReloadNotifier() notifier.INotifier {
	return ss.notifier
}

func (ss *Root) Reload() {
	ss.lock.Lock()
	providers := ss.providers
	ss.lock.Unlock()

	for _, provider := range providers {
		provider.Load()
	}
	ss.notifier.Notify()
}

func (ss *Root) GetProviders() []IConfigurationProvider {
	return ss.providers
}
