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

func TestYamlConfigurationSource_SimpleYaml(t *testing.T) {
	// 创建临时 YAML 文件
	yamlContent := `
key1: value1
key2: value2
key3: 123
key4: true
`
	tmpFile, err := os.CreateTemp("", "test*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.YamlConfigurationSource{
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

func TestYamlConfigurationSource_NestedStructure(t *testing.T) {
	yamlContent := `
database:
  host: localhost
  port: 3306
  name: testdb
app:
  name: myapp
  version: 1.0
`
	tmpFile, err := os.CreateTemp("", "test*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.YamlConfigurationSource{
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
	// YAML 中的 1.0 会被解析为整数 1
	assert.Equal(t, "1", manager.Get("app:version"))

	section := manager.GetSection("database")
	assert.Equal(t, "localhost", section.Get("host"))
	assert.Equal(t, "3306", section.Get("port"))
}

func TestYamlConfigurationSource_Array(t *testing.T) {
	yamlContent := `
servers:
  - host: server1
    port: 8080
  - host: server2
    port: 8081
`
	tmpFile, err := os.CreateTemp("", "test*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.YamlConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	// YAML 数组会被转换为 servers:0, servers:1 等键
	assert.Contains(t, manager.Get("servers:0:host"), "server")
}

func TestYamlConfigurationSource_NumberTypes(t *testing.T) {
	yamlContent := `
int_value: 42
float_value: 3.14
negative_int: -10
`
	tmpFile, err := os.CreateTemp("", "test*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.YamlConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	assert.Equal(t, "42", manager.Get("int_value"))
	assert.Contains(t, manager.Get("float_value"), "3.14")
	assert.Equal(t, "-10", manager.Get("negative_int"))
}

func TestYamlConfigurationSource_OptionalFile(t *testing.T) {
	source := &sources.YamlConfigurationSource{
		Path:           "nonexistent_file.yaml",
		Optional:       true,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	// 不应该 panic
	assert.NotPanics(t, func() {
		manager.AddSource(source)
	})
}

func TestYamlConfigurationSource_InvalidYaml(t *testing.T) {
	invalidYaml := `
key1: value1
  invalid: indentation
`
	tmpFile, err := os.CreateTemp("", "test*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(invalidYaml)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.YamlConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	// 应该能够处理无效的 YAML（会记录错误但不会 panic）
	assert.NotPanics(t, func() {
		manager.AddSource(source)
	})
}

func TestConvertYamlToConfigurationKV(t *testing.T) {
	yamlData := map[string]any{
		"key1": "value1",
		"key2": map[string]any{
			"subkey1": "subvalue1",
			"subkey2": 123,
		},
		"key3": []any{"item1", "item2"},
	}

	result, err := sources.ConvertYamlToConfigurationKV("", yamlData)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "value1", result.Get("key1"))
	assert.Equal(t, "subvalue1", result.Get("key2:subkey1"))
	assert.Equal(t, "123", result.Get("key2:subkey2"))
	assert.Equal(t, "item1", result.Get("key3:0"))
	assert.Equal(t, "item2", result.Get("key3:1"))
}
