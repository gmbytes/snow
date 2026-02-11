package configuration_test

import (
	"testing"

	"github.com/gmbytes/snow/core/configuration"
	"github.com/gmbytes/snow/core/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func TestManager_NewManager(t *testing.T) {
	manager := configuration.NewManager()
	assert.NotNil(t, manager)
	assert.Equal(t, 0, len(manager.GetSources()))
	assert.Equal(t, 0, len(manager.GetProviders()))
}

func TestManager_AddSource(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	manager.AddSource(source)

	assert.Equal(t, 1, len(manager.GetSources()))
	assert.Equal(t, 1, len(manager.GetProviders()))
	assert.Equal(t, "value1", manager.Get("key1"))
	assert.Equal(t, "value2", manager.Get("key2"))
}

func TestManager_Get(t *testing.T) {
	manager := configuration.NewManager()

	source1 := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1_from_source1",
		},
	}

	source2 := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1_from_source2",
			"key2": "value2",
		},
	}

	manager.AddSource(source1)
	manager.AddSource(source2)

	// 后添加的源优先级更高
	assert.Equal(t, "value1_from_source2", manager.Get("key1"))
	assert.Equal(t, "value2", manager.Get("key2"))
	assert.Equal(t, "", manager.Get("nonexistent"))
}

func TestManager_TryGet(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
		},
	}

	manager.AddSource(source)

	value, ok := manager.TryGet("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", value)

	value, ok = manager.TryGet("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestManager_Set(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
		},
	}

	manager.AddSource(source)
	manager.Set("key1", "newvalue1")
	manager.Set("key2", "value2")

	assert.Equal(t, "newvalue1", manager.Get("key1"))
	assert.Equal(t, "value2", manager.Get("key2"))
}

func TestManager_GetSection(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
			"database:port": "3306",
			"database:name": "testdb",
		},
	}

	manager.AddSource(source)

	section := manager.GetSection("database")
	assert.NotNil(t, section)
	assert.Equal(t, "database", section.GetKey())
	assert.Equal(t, "database", section.GetPath())
	assert.Equal(t, "localhost", section.Get("host"))
	assert.Equal(t, "3306", section.Get("port"))
	assert.Equal(t, "testdb", section.Get("name"))
}

func TestManager_GetChildren(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
			"database:port": "3306",
			"app:name":      "myapp",
			"app:version":   "1.0",
		},
	}

	manager.AddSource(source)

	children := manager.GetChildren()
	assert.Equal(t, 2, len(children))

	children = manager.GetChildrenByPath("database")
	assert.Equal(t, 2, len(children))
}

func TestManager_Reload(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1",
		},
	}

	manager.AddSource(source)

	reloadCount := 0
	manager.GetReloadNotifier().RegisterNotifyCallback(func() {
		reloadCount++
	})

	manager.Reload()
	assert.Equal(t, 1, reloadCount)
}

func TestManager_GetReloadNotifier(t *testing.T) {
	manager := configuration.NewManager()
	notifier := manager.GetReloadNotifier()
	assert.NotNil(t, notifier)

	notified := false
	notifier.RegisterNotifyCallback(func() {
		notified = true
	})

	manager.Reload()
	assert.True(t, notified)
}

func TestManager_MultipleSourcesPriority(t *testing.T) {
	manager := configuration.NewManager()

	// 添加第一个源
	source1 := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1_from_source1",
			"key2": "value2_from_source1",
		},
	}
	manager.AddSource(source1)

	// 添加第二个源（优先级更高）
	source2 := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"key1": "value1_from_source2",
			"key3": "value3_from_source2",
		},
	}
	manager.AddSource(source2)

	// 后添加的源优先级更高
	assert.Equal(t, "value1_from_source2", manager.Get("key1"))
	// key2 只在 source1 中存在
	assert.Equal(t, "value2_from_source1", manager.Get("key2"))
	// key3 只在 source2 中存在
	assert.Equal(t, "value3_from_source2", manager.Get("key3"))
}
