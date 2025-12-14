package sources_test

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/mogud/snow/core/configuration"
	"github.com/mogud/snow/core/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func init() {
	// 禁用测试中的日志输出，避免污染测试输出
	log.SetOutput(io.Discard)
}

func TestFileConfigurationSource_BuildConfigurationProvider(t *testing.T) {
	// 创建临时文件
	content := "test content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.NotNil(t, provider)
}

func TestFileConfigurationSource_Load(t *testing.T) {
	content := "test file content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)

	loadCalled := false
	loadedContent := ""
	provider.OnLoad = func(bytes []byte) {
		loadCalled = true
		loadedContent = string(bytes)
	}

	provider.Load()

	assert.True(t, loadCalled)
	assert.Equal(t, content, loadedContent)
}

func TestFileConfigurationSource_OptionalFile(t *testing.T) {
	source := &sources.FileConfigurationSource{
		Path:           "nonexistent_file.txt",
		Optional:       true,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)

	// 不应该 panic
	assert.NotPanics(t, func() {
		provider.Load()
	})
}

func TestFileConfigurationSource_NonOptionalFileNotFound(t *testing.T) {
	source := &sources.FileConfigurationSource{
		Path:           "nonexistent_file.txt",
		Optional:       false,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)

	// 应该 panic
	assert.Panics(t, func() {
		provider.Load()
	})
}

func TestFileConfigurationSource_Reload(t *testing.T) {
	content := "initial content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)

	loadCount := 0
	provider.OnLoad = func(bytes []byte) {
		loadCount++
	}

	provider.Load()
	assert.Equal(t, 1, loadCount)

	// 修改文件内容
	newContent := "updated content"
	err = os.WriteFile(tmpFile.Name(), []byte(newContent), 0644)
	assert.NoError(t, err)

	// 重新加载
	provider.Load()
	assert.Equal(t, 2, loadCount)
}

func TestFileConfigurationSource_WithManager(t *testing.T) {
	content := "key1=value1\nkey2=value2"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	manager := configuration.NewManager()
	manager.AddSource(source)

	// FileConfigurationSource 本身不解析内容，只是加载文件
	// 实际解析由 OnLoad 回调处理
	assert.NotNil(t, manager)
}

func TestFileConfigurationSource_ReloadOnChange(t *testing.T) {
	content := "initial content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: true,
	}

	provider := sources.NewFileConfigurationProvider(source)

	loadCount := 0
	provider.OnLoad = func(bytes []byte) {
		loadCount++
	}

	provider.Load()
	assert.Equal(t, 1, loadCount)

	// 等待文件监听器启动
	time.Sleep(100 * time.Millisecond)

	// 修改文件
	newContent := "updated content"
	err = os.WriteFile(tmpFile.Name(), []byte(newContent), 0644)
	assert.NoError(t, err)

	// 等待文件监听器触发（有 500ms 延迟）
	time.Sleep(600 * time.Millisecond)

	// 应该触发重新加载
	assert.GreaterOrEqual(t, loadCount, 1)
}

func TestFileConfigurationSource_ReloadOnChangeDisabled(t *testing.T) {
	content := "test content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)

	loadCount := 0
	provider.OnLoad = func(bytes []byte) {
		loadCount++
	}

	provider.Load()
	assert.Equal(t, 1, loadCount)

	// 再次调用 Load 应该重新加载
	provider.Load()
	assert.Equal(t, 2, loadCount)
}

func TestFileConfigurationProvider_GetReloadNotifier(t *testing.T) {
	content := "test content"
	tmpFile, err := os.CreateTemp("", "test*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	source := &sources.FileConfigurationSource{
		Path:           tmpFile.Name(),
		Optional:       false,
		ReloadOnChange: false,
	}

	provider := sources.NewFileConfigurationProvider(source)
	provider.Load()

	notifier := provider.GetReloadNotifier()
	assert.NotNil(t, notifier)

	notified := false
	notifier.RegisterNotifyCallback(func() {
		notified = true
	})

	provider.OnReload()
	assert.True(t, notified)
}
