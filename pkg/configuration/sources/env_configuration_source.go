package sources

import (
	"os"
	"strings"

	"github.com/gmbytes/snow/pkg/configuration"
)

var _ configuration.IConfigurationSource = (*EnvironmentConfigurationSource)(nil)

// EnvironmentConfigurationSource 从环境变量中读取配置。
// 命名约定：APP_NODE_LOCALIP → Node:LocalIP（下划线转为层级分隔符冒号）。
// Prefix 可自定义（默认 "APP_"），设为空字符串则读取所有环境变量。
type EnvironmentConfigurationSource struct {
	Prefix string // 环境变量前缀过滤（默认 "APP_"）
}

// BuildConfigurationProvider 读取环境变量并构建配置提供者。
func (ss *EnvironmentConfigurationSource) BuildConfigurationProvider(
	_ configuration.IConfigurationBuilder,
) configuration.IConfigurationProvider {
	data := make(map[string]string)

	for _, env := range os.Environ() {
		key, value, found := strings.Cut(env, "=")
		if !found {
			continue
		}

		if ss.Prefix != "" && !strings.HasPrefix(key, ss.Prefix) {
			continue
		}

		// 去除前缀，将 _ 分隔的片段转换为 : 层级分隔符
		trimmed := strings.TrimPrefix(key, ss.Prefix)
		configKey := envKeyToConfigKey(trimmed)
		data[configKey] = value
	}

	return NewMemoryConfigurationProvider(&MemoryConfigurationSource{InitData: data})
}

// envKeyToConfigKey 将环境变量名（去除前缀后）转换为配置键。
// 规则：按 _ 分割，每个片段首字母大写，用 : 拼接。
// 示例：NODE_LOCALIP → Node:LocalIP（但整体 title-case 转换）
// 实际：每段做 Title-case（首字母大写，其余小写），例：LOCALIP → Localip。
// 为保持与 APP_NODE_LOCALIP → Node:LocalIP 的一致性，对每段做首字母大写处理。
func envKeyToConfigKey(envKey string) string {
	if envKey == "" {
		return ""
	}

	parts := strings.Split(envKey, "_")
	for i, p := range parts {
		parts[i] = toTitleSegment(p)
	}
	return strings.Join(parts, ":")
}

// toTitleSegment 将字符串首字母大写，其余字母小写。
func toTitleSegment(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
