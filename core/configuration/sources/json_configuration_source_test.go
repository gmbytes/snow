package sources_test

import (
	"io"
	"log"
	"os"
	"testing"

	"github.com/mogud/snow/core/configuration"
	"github.com/mogud/snow/core/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func init() {
	// 禁用测试中的日志输出，避免污染测试输出
	log.SetOutput(io.Discard)
}

func TestJsonConfigurationSource_BuildConfigurationProvider(t *testing.T) {
	jsonContent := `{
		"key1": "value1",
		"key2": "value2"
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.NotNil(t, provider)
}

func TestJsonConfigurationSource_SimpleJson(t *testing.T) {
	jsonContent := `{
		"key1": "value1",
		"key2": "value2",
		"key3": 123,
		"key4": true
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "value1", manager.Get("key1"))
	assert.Equal(t, "value2", manager.Get("key2"))
	assert.Equal(t, "123", manager.Get("key3"))
	assert.Equal(t, "true", manager.Get("key4"))
}

func TestJsonConfigurationSource_NestedStructure(t *testing.T) {
	jsonContent := `{
		"database": {
			"host": "localhost",
			"port": 3306,
			"name": "testdb"
		},
		"app": {
			"name": "myapp",
			"version": "1.0"
		}
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "localhost", manager.Get("database:host"))
	assert.Equal(t, "3306", manager.Get("database:port"))
	assert.Equal(t, "testdb", manager.Get("database:name"))
	assert.Equal(t, "myapp", manager.Get("app:name"))
	assert.Equal(t, "1.0", manager.Get("app:version"))

	section := manager.GetSection("database")
	assert.Equal(t, "localhost", section.Get("host"))
	assert.Equal(t, "3306", section.Get("port"))
}

func TestJsonConfigurationSource_Array(t *testing.T) {
	jsonContent := `{
		"servers": [
			"server1",
			"server2",
			"server3"
		],
		"ports": [8080, 8081, 8082]
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "server1", manager.Get("servers:0"))
	assert.Equal(t, "server2", manager.Get("servers:1"))
	assert.Equal(t, "server3", manager.Get("servers:2"))
	assert.Equal(t, "8080", manager.Get("ports:0"))
	assert.Equal(t, "8081", manager.Get("ports:1"))
	assert.Equal(t, "8082", manager.Get("ports:2"))
}

func TestJsonConfigurationSource_NestedArray(t *testing.T) {
	jsonContent := `{
		"servers": [
			{
				"host": "server1",
				"port": 8080
			},
			{
				"host": "server2",
				"port": 8081
			}
		]
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "server1", manager.Get("servers:0:host"))
	assert.Equal(t, "8080", manager.Get("servers:0:port"))
	assert.Equal(t, "server2", manager.Get("servers:1:host"))
	assert.Equal(t, "8081", manager.Get("servers:1:port"))
}

func TestJsonConfigurationSource_NumberTypes(t *testing.T) {
	jsonContent := `{
		"int_value": 42,
		"float_value": 3.14,
		"negative_int": -10,
		"zero": 0
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "42", manager.Get("int_value"))
	assert.Equal(t, "3.140000", manager.Get("float_value"))
	assert.Equal(t, "-10", manager.Get("negative_int"))
	assert.Equal(t, "0", manager.Get("zero"))
}

func TestJsonConfigurationSource_Boolean(t *testing.T) {
	jsonContent := `{
		"enabled": true,
		"disabled": false
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "true", manager.Get("enabled"))
	assert.Equal(t, "false", manager.Get("disabled"))
}

func TestJsonConfigurationSource_WithComments(t *testing.T) {
	jsonContent := `{
		// This is a comment
		"key1": "value1",
		/* This is a block comment */
		"key2": "value2"
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "value1", manager.Get("key1"))
	assert.Equal(t, "value2", manager.Get("key2"))
}

func TestJsonConfigurationSource_OptionalFile(t *testing.T) {
	source := &sources.JsonConfigurationSource{
		Path:           "nonexistent_file.json",
		Optional:       true,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	// 不应该 panic
	assert.NotPanics(t, func() {
		manager.AddSource(source)
	})
}

func TestJsonConfigurationSource_InvalidJson(t *testing.T) {
	invalidJson := `{
		"key1": "value1"
		"key2": "value2"  // missing comma
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(invalidJson)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	// 应该能够处理无效的 JSON（会记录错误但不会 panic）
	assert.NotPanics(t, func() {
		manager.AddSource(source)
	})
}

func TestConvertJsonToConfigurationKV(t *testing.T) {
	jsonStr := `{
		"key1": "value1",
		"key2": {
			"subkey1": "subvalue1",
			"subkey2": 123
		},
		"key3": ["item1", "item2"]
	}`

	result, err := sources.ConvertJsonToConfigurationKV("", jsonStr)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "value1", result.Get("key1"))
	assert.Equal(t, "subvalue1", result.Get("key2:subkey1"))
	assert.Equal(t, "123", result.Get("key2:subkey2"))
	assert.Equal(t, "item1", result.Get("key3:0"))
	assert.Equal(t, "item2", result.Get("key3:1"))
}

func TestConvertJsonToConfigurationKV_WithHead(t *testing.T) {
	jsonStr := `{
		"key1": "value1",
		"key2": "value2"
	}`

	result, err := sources.ConvertJsonToConfigurationKV("prefix", jsonStr)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "value1", result.Get("prefix:key1"))
	assert.Equal(t, "value2", result.Get("prefix:key2"))
}

func TestConvertJsonToConfigurationKV_InvalidJson(t *testing.T) {
	invalidJson := `{
		"key1": "value1"
		invalid syntax
	}`

	result, err := sources.ConvertJsonToConfigurationKV("", invalidJson)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestJsonConfigurationSource_EmptyObject(t *testing.T) {
	jsonContent := `{}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	// 空对象应该不会导致错误
	assert.NotNil(t, manager)
}

func TestJsonConfigurationSource_ComplexNested(t *testing.T) {
	jsonContent := `{
		"app": {
			"database": {
				"host": "localhost",
				"port": 3306
			},
			"cache": {
				"host": "redis",
				"port": 6379
			}
		}
	}`
	tmpFile, err := os.CreateTemp("", "test*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.JsonConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "localhost", manager.Get("app:database:host"))
	assert.Equal(t, "3306", manager.Get("app:database:port"))
	assert.Equal(t, "redis", manager.Get("app:cache:host"))
	assert.Equal(t, "6379", manager.Get("app:cache:port"))

	appSection := manager.GetSection("app")
	dbSection := appSection.GetSection("database")
	assert.Equal(t, "localhost", dbSection.Get("host"))
}
