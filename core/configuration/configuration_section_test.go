package configuration_test

import (
	"testing"

	"github.com/mogud/snow/core/configuration"
	"github.com/mogud/snow/core/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func TestSection_Get(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
			"database:port": "3306",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")

	assert.Equal(t, "localhost", section.Get("host"))
	assert.Equal(t, "3306", section.Get("port"))
	assert.Equal(t, "", section.Get("nonexistent"))
}

func TestSection_TryGet(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")

	value, ok := section.TryGet("host")
	assert.True(t, ok)
	assert.Equal(t, "localhost", value)

	value, ok = section.TryGet("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestSection_Set(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:host": "localhost",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")

	section.Set("port", "3306")
	assert.Equal(t, "3306", section.Get("port"))
}

func TestSection_GetSection(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:mysql:host": "localhost",
			"database:mysql:port": "3306",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")
	subSection := section.GetSection("mysql")

	assert.NotNil(t, subSection)
	assert.Equal(t, "mysql", subSection.GetKey())
	assert.Equal(t, "database:mysql", subSection.GetPath())
	assert.Equal(t, "localhost", subSection.Get("host"))
}

func TestSection_GetChildren(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:mysql:host":    "localhost",
			"database:postgres:host": "localhost",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")
	children := section.GetChildren()

	assert.Equal(t, 2, len(children))
}

func TestSection_GetChildrenByPath(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database:mysql:host": "localhost",
			"database:mysql:port": "3306",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")
	children := section.GetChildrenByPath("mysql")

	assert.Equal(t, 2, len(children))
}

func TestSection_GetKey(t *testing.T) {
	manager := configuration.NewManager()
	section := manager.GetSection("database")
	assert.Equal(t, "database", section.GetKey())

	subSection := section.GetSection("mysql")
	assert.Equal(t, "mysql", subSection.GetKey())
}

func TestSection_GetPath(t *testing.T) {
	manager := configuration.NewManager()
	section := manager.GetSection("database")
	assert.Equal(t, "database", section.GetPath())

	subSection := section.GetSection("mysql")
	assert.Equal(t, "database:mysql", subSection.GetPath())
}

func TestSection_GetValue(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"database":      "testdb",
			"database:host": "localhost",
		},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")

	value, ok := section.GetValue()
	assert.True(t, ok)
	assert.Equal(t, "testdb", value)
}

func TestSection_SetValue(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{},
	}

	manager.AddSource(source)
	section := manager.GetSection("database")

	section.SetValue("", "testdb")
	value, ok := section.GetValue()
	assert.True(t, ok)
	assert.Equal(t, "testdb", value)
}

func TestSection_NestedSections(t *testing.T) {
	manager := configuration.NewManager()

	source := &sources.MemoryConfigurationSource{
		InitData: map[string]string{
			"app:database:host": "localhost",
			"app:database:port": "3306",
			"app:cache:host":    "redis",
			"app:cache:port":    "6379",
		},
	}

	manager.AddSource(source)

	appSection := manager.GetSection("app")
	assert.Equal(t, "app", appSection.GetKey())

	dbSection := appSection.GetSection("database")
	assert.Equal(t, "database", dbSection.GetKey())
	assert.Equal(t, "app:database", dbSection.GetPath())
	assert.Equal(t, "localhost", dbSection.Get("host"))
	assert.Equal(t, "3306", dbSection.Get("port"))

	cacheSection := appSection.GetSection("cache")
	assert.Equal(t, "cache", cacheSection.GetKey())
	assert.Equal(t, "app:cache", cacheSection.GetPath())
	assert.Equal(t, "redis", cacheSection.Get("host"))
	assert.Equal(t, "6379", cacheSection.Get("port"))
}
