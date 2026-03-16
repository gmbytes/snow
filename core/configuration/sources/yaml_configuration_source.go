package sources

import (
	"fmt"
	"log"

	"github.com/gmbytes/snow/core/configuration"
	"gopkg.in/yaml.v3"
)

var _ configuration.IConfigurationSource = (*YamlConfigurationSource)(nil)

type YamlConfigurationSource struct {
	Path           string
	Optional       bool
	ReloadOnChange bool
}

func (ss *YamlConfigurationSource) BuildConfigurationProvider(_ configuration.IConfigurationBuilder) configuration.IConfigurationProvider {
	return NewYamlConfigurationProvider(ss)
}

var _ configuration.IConfigurationProvider = (*YamlConfigurationProvider)(nil)

type YamlConfigurationProvider struct {
	*FileConfigurationProvider
}

func NewYamlConfigurationProvider(source *YamlConfigurationSource) configuration.IConfigurationProvider {
	provider := &YamlConfigurationProvider{
		FileConfigurationProvider: NewFileConfigurationProvider(&FileConfigurationSource{
			Path:           source.Path,
			Optional:       source.Optional,
			ReloadOnChange: source.ReloadOnChange,
		}),
	}
	provider.OnLoad = provider.OnLoadYaml
	return provider
}

func (ss *YamlConfigurationProvider) OnLoadYaml(bytes []byte) {
	var yamlData any
	if err := yaml.Unmarshal(bytes, &yamlData); err != nil {
		log.Printf("load yaml: %v", err)
		ss.Replace(configuration.NewCaseInsensitiveStringMap[string]())
		return
	}

	newMap, err := ConvertYamlToConfigurationKV("", yamlData)
	if err != nil {
		log.Printf("convert yaml to configuration: %v", err)
		ss.Replace(configuration.NewCaseInsensitiveStringMap[string]())
		return
	}

	ss.Replace(newMap)
}

func ConvertYamlToConfigurationKV(head string, yamlData any) (*configuration.CaseInsensitiveStringMap[string], error) {
	newMap := configuration.NewCaseInsensitiveStringMap[string]()

	// 处理顶层可能是 map 或直接是其他类型的情况
	switch data := yamlData.(type) {
	case map[string]any:
		for key, value := range data {
			if len(head) == 0 {
				if err := fillMapFromYaml(newMap, key, value); err != nil {
					return nil, err
				}
			} else {
				if err := fillMapFromYaml(newMap, fmt.Sprintf("%s:%s", head, key), value); err != nil {
					return nil, err
				}
			}
		}
	case map[interface{}]interface{}:
		for k, value := range data {
			key := fmt.Sprintf("%v", k)
			if len(head) == 0 {
				if err := fillMapFromYaml(newMap, key, value); err != nil {
					return nil, err
				}
			} else {
				if err := fillMapFromYaml(newMap, fmt.Sprintf("%s:%s", head, key), value); err != nil {
					return nil, err
				}
			}
		}
	default:
		// 如果顶层不是 map，将其作为根键值处理
		if err := fillMapFromYaml(newMap, head, yamlData); err != nil {
			return nil, err
		}
	}
	return newMap, nil
}

func fillMapFromYaml(m *configuration.CaseInsensitiveStringMap[string], key string, value any) error {
	switch v := value.(type) {
	case string:
		m.Add(key, v)
	case map[string]any:
		for k, val := range v {
			if err := fillMapFromYaml(m, fmt.Sprintf("%s:%s", key, k), val); err != nil {
				return err
			}
		}
	case map[interface{}]interface{}:
		// YAML 解析可能返回 map[interface{}]interface{}
		for k, val := range v {
			keyStr := fmt.Sprintf("%v", k)
			if err := fillMapFromYaml(m, fmt.Sprintf("%s:%s", key, keyStr), val); err != nil {
				return err
			}
		}
	case []any:
		for i, val := range v {
			if err := fillMapFromYaml(m, fmt.Sprintf("%s:%d", key, i), val); err != nil {
				return err
			}
		}
	case int:
		m.Add(key, fmt.Sprintf("%d", v))
	case int8:
		m.Add(key, fmt.Sprintf("%d", v))
	case int16:
		m.Add(key, fmt.Sprintf("%d", v))
	case int32:
		m.Add(key, fmt.Sprintf("%d", v))
	case int64:
		m.Add(key, fmt.Sprintf("%d", v))
	case uint:
		m.Add(key, fmt.Sprintf("%d", v))
	case uint8:
		m.Add(key, fmt.Sprintf("%d", v))
	case uint16:
		m.Add(key, fmt.Sprintf("%d", v))
	case uint32:
		m.Add(key, fmt.Sprintf("%d", v))
	case uint64:
		m.Add(key, fmt.Sprintf("%d", v))
	case float32:
		m.Add(key, fmt.Sprintf("%g", v))
	case float64:
		// 检查是否为整数
		n := int64(v)
		if v == float64(n) {
			m.Add(key, fmt.Sprintf("%d", n))
			return nil
		}
		m.Add(key, fmt.Sprintf("%g", v))
	case bool:
		m.Add(key, fmt.Sprintf("%t", v))
	case nil:
		m.Add(key, "")
	default:
		return fmt.Errorf("invalid type: %T => %v", v, v)
	}
	return nil
}
