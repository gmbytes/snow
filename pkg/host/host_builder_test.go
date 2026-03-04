package host_test

import (
	"reflect"
	"testing"

	internal "github.com/gmbytes/snow/internal/host"
	"github.com/gmbytes/snow/pkg/configuration"
	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/injection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tyOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

type iBuilderTestSvc interface{ Name() string }
type builderTestSvcImpl struct{ name string }

func (b *builderTestSvcImpl) Name() string { return b.name }

type iBuilderTestOther interface{ Val() int }
type builderTestOtherImpl struct{ val int }

func (b *builderTestOtherImpl) Val() int { return b.val }

var _ host.IBuilder = (*builderAdapter)(nil)

type builderAdapter struct {
	col      injection.IRoutineCollection
	provider injection.IRoutineProvider
	cfg      configuration.IConfigurationManager
}

func (b *builderAdapter) GetRoutineCollection() injection.IRoutineCollection           { return b.col }
func (b *builderAdapter) GetRoutineProvider() injection.IRoutineProvider               { return b.provider }
func (b *builderAdapter) GetConfigurationManager() configuration.IConfigurationManager { return b.cfg }
func (b *builderAdapter) Build() host.IHost                                            { return nil }

func newBuilder() (*builderAdapter, injection.IRoutineCollection, injection.IRoutineProvider) {
	col := internal.NewRoutineCollection()
	provider := internal.NewProvider(col, nil)
	cfg := configuration.NewManager()
	return &builderAdapter{col: col, provider: provider, cfg: cfg}, col, provider
}

func TestAddSingleton_RegistersDescriptor_ExpectSingletonLifetime(t *testing.T) {
	b, col, _ := newBuilder()

	host.AddSingletonFactory[iBuilderTestSvc](b, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "hello"}
	})

	desc := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	require.NotNil(t, desc)
	assert.Equal(t, injection.Singleton, desc.Lifetime)
	assert.Equal(t, tyOf[iBuilderTestSvc](), desc.TyKey)
}

func TestAddTransient_RegistersDescriptor_ExpectTransientLifetime(t *testing.T) {
	b, col, _ := newBuilder()

	host.AddTransientFactory[iBuilderTestSvc](b, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "t"}
	})

	desc := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	require.NotNil(t, desc)
	assert.Equal(t, injection.Transient, desc.Lifetime)
}

func TestAddScoped_RegistersDescriptor_ExpectScopedLifetime(t *testing.T) {
	b, col, _ := newBuilder()

	host.AddScopedFactory[iBuilderTestSvc](b, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "s"}
	})

	desc := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	require.NotNil(t, desc)
	assert.Equal(t, injection.Scoped, desc.Lifetime)
}

func TestAddVariantSingleton_RegistersWithInterfaceKey(t *testing.T) {
	b, col, _ := newBuilder()

	host.AddVariantSingletonFactory[iBuilderTestSvc, *builderTestSvcImpl](b,
		func(_ injection.IRoutineScope) *builderTestSvcImpl {
			return &builderTestSvcImpl{name: "variant"}
		},
	)

	desc := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	require.NotNil(t, desc)
	assert.Equal(t, tyOf[iBuilderTestSvc](), desc.TyKey)
	assert.Equal(t, tyOf[*builderTestSvcImpl](), desc.TyImpl)
}

func TestAddKeyedSingleton_RegistersWithKey_ExpectKeyedLookup(t *testing.T) {
	b, col, _ := newBuilder()
	key := "myKey"

	host.AddKeyedSingletonFactory[iBuilderTestSvc](b, key, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "keyed"}
	})

	descDefault := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	assert.Nil(t, descDefault, "default key 应查不到")

	descKeyed := col.GetKeyedDescriptor(key, tyOf[iBuilderTestSvc]())
	require.NotNil(t, descKeyed)
	assert.Equal(t, injection.Singleton, descKeyed.Lifetime)
}

func TestGetRoutine_WhenNil_ReturnsZero(t *testing.T) {
	provider := internal.NewProvider(internal.NewRoutineCollection(), nil)

	var result iBuilderTestSvc = host.GetRoutine[iBuilderTestSvc](provider)
	assert.Nil(t, result)
}

func TestGetRoutine_WhenTypeMatch_ReturnsValue(t *testing.T) {
	col := internal.NewRoutineCollection()
	expected := &builderTestSvcImpl{name: "found"}
	col.AddDescriptor(&injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    tyOf[iBuilderTestSvc](),
		Factory:  func(_ injection.IRoutineScope) any { return expected },
	})
	provider := internal.NewProvider(col, nil)

	result := host.GetRoutine[iBuilderTestSvc](provider)
	require.NotNil(t, result)
	assert.Equal(t, "found", result.Name())
}

func TestAddSingletonFactory_FactoryCalledOnResolve_ExpectCorrectValue(t *testing.T) {
	b, _, provider := newBuilder()

	host.AddSingletonFactory[iBuilderTestSvc](b, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "resolved"}
	})

	result := host.GetRoutine[iBuilderTestSvc](provider)
	require.NotNil(t, result)
	assert.Equal(t, "resolved", result.Name())
}

func TestAddMultipleDescriptors_AllRetrievable(t *testing.T) {
	b, col, _ := newBuilder()

	host.AddSingletonFactory[iBuilderTestSvc](b, func(_ injection.IRoutineScope) iBuilderTestSvc {
		return &builderTestSvcImpl{name: "svc"}
	})
	host.AddSingletonFactory[iBuilderTestOther](b, func(_ injection.IRoutineScope) iBuilderTestOther {
		return &builderTestOtherImpl{val: 42}
	})

	all := col.GetDescriptors()
	assert.Len(t, all, 2)

	descSvc := col.GetDescriptor(tyOf[iBuilderTestSvc]())
	descOther := col.GetDescriptor(tyOf[iBuilderTestOther]())
	require.NotNil(t, descSvc)
	require.NotNil(t, descOther)
}
