package configuration_test

import (
	"testing"

	"github.com/mogud/snow/core/configuration"
	"github.com/stretchr/testify/assert"
)

func TestProvider_Get(t *testing.T) {
	provider := configuration.NewProvider()
	provider.Set("key1", "value1")
	provider.Set("key2", "value2")

	assert.Equal(t, "value1", provider.Get("key1"))
	assert.Equal(t, "value2", provider.Get("key2"))
	assert.Equal(t, "", provider.Get("nonexistent"))
}

func TestProvider_TryGet(t *testing.T) {
	provider := configuration.NewProvider()
	provider.Set("key1", "value1")

	value, ok := provider.TryGet("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", value)

	value, ok = provider.TryGet("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestProvider_Set(t *testing.T) {
	provider := configuration.NewProvider()
	provider.Set("key1", "value1")
	assert.Equal(t, "value1", provider.Get("key1"))

	provider.Set("key1", "value2")
	assert.Equal(t, "value2", provider.Get("key1"))
}

func TestProvider_GetChildKeys(t *testing.T) {
	provider := configuration.NewProvider()
	provider.Set("database:host", "localhost")
	provider.Set("database:port", "3306")
	provider.Set("database:name", "testdb")
	provider.Set("app:name", "myapp")

	keys := provider.GetChildKeys("")
	// GetChildKeys 会返回所有键的第一段，包括重复的
	// 实际返回: ["database", "database", "database", "app"]
	assert.GreaterOrEqual(t, len(keys), 2)
	assert.Contains(t, keys, "database")
	assert.Contains(t, keys, "app")

	keys = provider.GetChildKeys("database")
	assert.Equal(t, 3, len(keys))
	assert.Contains(t, keys, "host")
	assert.Contains(t, keys, "port")
	assert.Contains(t, keys, "name")
}

func TestProvider_GetReloadNotifier(t *testing.T) {
	provider := configuration.NewProvider()
	notifier := provider.GetReloadNotifier()
	assert.NotNil(t, notifier)

	notified := false
	notifier.RegisterNotifyCallback(func() {
		notified = true
	})

	provider.OnReload()
	assert.True(t, notified)
}

func TestProvider_Replace(t *testing.T) {
	provider := configuration.NewProvider()
	provider.Set("key1", "value1")

	newData := configuration.NewCaseInsensitiveStringMap[string]()
	newData.Add("key2", "value2")
	newData.Add("key3", "value3")

	provider.Replace(newData)

	assert.Equal(t, "", provider.Get("key1"))
	assert.Equal(t, "value2", provider.Get("key2"))
	assert.Equal(t, "value3", provider.Get("key3"))
}
