package configuration

import (
	"sync"

	"github.com/mogud/snow/core/notifier"
)

var _ IConfigurationManager = (*Manager)(nil)

type Manager struct {
	lock       sync.Mutex
	properties map[string]any
	sources    []IConfigurationSource
	providers  []IConfigurationProvider
	notifier   *Notifier
}

func NewManager() *Manager {
	return &Manager{
		properties: make(map[string]any),
		sources:    make([]IConfigurationSource, 0),
		providers:  make([]IConfigurationProvider, 0),
		notifier:   NewNotifier(),
	}
}

func (ss *Manager) Get(key string) string {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for i := len(ss.providers) - 1; i >= 0; i-- {
		if value, ok := ss.providers[i].TryGet(key); ok {
			return value
		}
	}
	return ""
}

func (ss *Manager) TryGet(key string) (value string, ok bool) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for i := len(ss.providers) - 1; i >= 0; i-- {
		if value, ok = ss.providers[i].TryGet(key); ok {
			return value, true
		}
	}
	return
}

func (ss *Manager) Set(key string, value string) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	for _, provider := range ss.providers {
		provider.Set(key, value)
	}
}

func (ss *Manager) GetSection(key string) IConfigurationSection {
	return NewSection(ss, key)
}

func (ss *Manager) GetChildren() []IConfigurationSection {
	return ss.GetChildrenByPath("")
}

func (ss *Manager) GetChildrenByPath(path string) []IConfigurationSection {
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

func (ss *Manager) GetReloadNotifier() notifier.INotifier {
	return ss.notifier
}

func (ss *Manager) Reload() {
	defer ss.notifier.Notify()

	ss.lock.Lock()
	defer ss.lock.Unlock()

	for _, provider := range ss.providers {
		provider.Load()
	}
}

func (ss *Manager) GetProviders() []IConfigurationProvider {
	return ss.providers
}

func (ss *Manager) GetProperties() map[string]any {
	return ss.properties
}

func (ss *Manager) GetSources() []IConfigurationSource {
	return ss.sources
}

func (ss *Manager) AddSource(source IConfigurationSource) {
	newProvider := source.BuildConfigurationProvider(ss)
	newProvider.Load()

	defer ss.notifier.Notify()

	ss.lock.Lock()
	defer ss.lock.Unlock()

	ss.sources = append(ss.sources, source)
	ss.providers = append(ss.providers, newProvider)

	newProvider.GetReloadNotifier().RegisterNotifyCallback(ss.notifier.Notify)
}

func (ss *Manager) BuildConfigurationRoot() IConfigurationRoot {
	return ss
}
