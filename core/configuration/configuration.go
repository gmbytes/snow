package configuration

import (
	"github.com/gmbytes/snow/core/notifier"
)

type IConfigurationSource interface {
	BuildConfigurationProvider(builder IConfigurationBuilder) IConfigurationProvider
}

type IConfigurationProvider interface {
	Get(key string) string
	TryGet(key string) (value string, ok bool)
	Set(key string, value string)
	GetReloadNotifier() notifier.INotifier
	Load()
	GetChildKeys(parentPath string) []string
}

type IConfigurationBuilder interface {
	GetProperties() map[string]any
	GetSources() []IConfigurationSource
	AddSource(source IConfigurationSource)
	BuildConfigurationRoot() IConfigurationRoot
}

type IConfiguration interface {
	Get(key string) string
	TryGet(key string) (value string, ok bool)
	Set(key string, value string)
	GetSection(key string) IConfigurationSection
	GetChildren() []IConfigurationSection
	GetChildrenByPath(path string) []IConfigurationSection
	GetReloadNotifier() notifier.INotifier
}

type IConfigurationRoot interface {
	IConfiguration

	Reload()
	GetProviders() []IConfigurationProvider
}

type IConfigurationSection interface {
	IConfiguration

	GetKey() string
	GetPath() string
	GetValue() (string, bool)
	SetValue(key string, value string)
}

type IConfigurationManager interface {
	IConfigurationBuilder
	IConfigurationRoot
}
