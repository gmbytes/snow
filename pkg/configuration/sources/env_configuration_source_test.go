package sources_test

import (
	"testing"

	"github.com/gmbytes/snow/pkg/configuration"
	"github.com/gmbytes/snow/pkg/configuration/sources"
	"github.com/stretchr/testify/assert"
)

func TestEnvSource_WhenPrefixMatch_ReturnsValues(t *testing.T) {
	t.Setenv("APP_FOO", "bar")

	source := &sources.EnvironmentConfigurationSource{Prefix: "APP_"}
	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.Equal(t, "bar", provider.Get("Foo"))
}

func TestEnvSource_WhenNestedKey_MapsHierarchy(t *testing.T) {
	t.Setenv("APP_NODE_LOCALIP", "1.2.3.4")

	source := &sources.EnvironmentConfigurationSource{Prefix: "APP_"}
	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.Equal(t, "1.2.3.4", provider.Get("Node:Localip"))
}

func TestEnvSource_WhenNoPrefix_ReadsAll(t *testing.T) {
	t.Setenv("SNOW_TEST_NOPREFIX_KEY", "hello")

	source := &sources.EnvironmentConfigurationSource{Prefix: ""}
	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	// 无前缀时读取所有，SNOW_TEST_NOPREFIX_KEY → Snow:Test:Noprefix:Key
	assert.Equal(t, "hello", provider.Get("Snow:Test:Noprefix:Key"))
}

func TestEnvSource_WhenNoMatch_ReturnsEmpty(t *testing.T) {
	// 使用一个不存在的前缀
	source := &sources.EnvironmentConfigurationSource{Prefix: "XYZNOEXIST_"}
	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.Equal(t, "", provider.Get("Foo"))
	assert.Equal(t, "", provider.Get("Bar"))
}

func TestEnvSource_WhenMultiLevel_MapsCorrectly(t *testing.T) {
	t.Setenv("APP_A_B_C", "x")

	source := &sources.EnvironmentConfigurationSource{Prefix: "APP_"}
	manager := configuration.NewManager()
	provider := source.BuildConfigurationProvider(manager)

	assert.Equal(t, "x", provider.Get("A:B:C"))
}
