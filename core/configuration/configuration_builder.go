package configuration

var _ IConfigurationBuilder = (*Builder)(nil)

type Builder struct {
	properties map[string]any
	sources    []IConfigurationSource
}

func NewBuilder() *Builder {
	return &Builder{
		properties: make(map[string]any),
		sources:    make([]IConfigurationSource, 0),
	}
}

func (ss *Builder) GetProperties() map[string]any {
	return ss.properties
}

func (ss *Builder) GetSources() []IConfigurationSource {
	return ss.sources
}

func (ss *Builder) AddSource(source IConfigurationSource) {
	ss.sources = append(ss.sources, source)
}

func (ss *Builder) BuildConfigurationRoot() IConfigurationRoot {
	providers := make([]IConfigurationProvider, 0, len(ss.sources))
	for _, source := range ss.sources {
		providers = append(providers, source.BuildConfigurationProvider(ss))
	}
	return NewConfigurationRoot(providers)
}
