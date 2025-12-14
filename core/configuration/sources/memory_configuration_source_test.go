package sources_test

import (
	"testing"

	"github.com/mogud/snow/core/configuration"
	"github.com/mogud/snow/core/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func TestMemoryConfigurationSource_BuildConfigurationProvider(t *testing.T) {
	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.NotNil(t, provider)
	assert.Equal(t, "value1", provider.Get("key1"))
	assert.Equal(t, "value2", provider.Get("key2"))
}

func TestMemoryConfigurationSource_EmptyInitData(t *testing.T) {
	source := &sources.MemoryConfigurationSource{
		InitData: nil,
	}

	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.NotNil(t, provider)
	assert.Equal(t, "", provider.Get("key1"))
}

func TestMemoryConfigurationSource_WithManager(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
			"database:port": "3306",
			"app:name":      "myapp",
		},
	}

	manager.AddSource(source)

	assert.Equal(t, "localhost", manager.Get("database:host"))
	assert.Equal(t, "3306", manager.Get("database:port"))
	assert.Equal(t, "myapp", manager.Get("app:name"))

	section := manager.GetSection("database")
	assert.Equal(t, "localhost", section.Get("host"))
	assert.Equal(t, "3306", section.Get("port"))
}

func TestMemoryConfigurationSource_Reload(t *testing.T) {
	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
		},
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	reloadCount := 0
	manager.GetReloadNotifier().RegisterNotifyCallback(func() {
		reloadCount++
	})

	manager.Reload()
	assert.Equal(t, 1, reloadCount)
}
