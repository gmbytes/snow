package internal

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gmbytes/snow/pkg/injection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 测试用辅助类型（与 routine_collection_test.go 不重复，用 provider 前缀区分）
// ---------------------------------------------------------------------------

type iProviderFoo interface{ Foo() string }

type providerFooImpl struct{ val string }

func (p *providerFooImpl) Foo() string { return p.val }

type iProviderBar interface{ Bar() string }

type providerBarImpl struct{ val string }

func (p *providerBarImpl) Bar() string { return p.val }

// 计数工厂：每次调用递增
type factoryCounter struct {
	mu    sync.Mutex
	count int32
}

func (fc *factoryCounter) inc() int32 {
	return atomic.AddInt32(&fc.count, 1)
}

func (fc *factoryCounter) total() int32 {
	return atomic.LoadInt32(&fc.count)
}

// ---------------------------------------------------------------------------
// 辅助：建立一个只有单条描述符的 collection + provider
// ---------------------------------------------------------------------------

func newProviderWith(desc *injection.RoutineDescriptor) *RoutineProvider {
	col := NewRoutineCollection()
	col.AddDescriptor(desc)
	return NewProvider(col, nil)
}

func providerTyOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetRoutine_WhenSingleton_ReturnsSameInstance
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetRoutine_WhenSingleton_ReturnsSameInstance(t *testing.T) {
	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    providerTyOf[iProviderFoo](),
		Factory: func(_ injection.IRoutineScope) any {
			return &providerFooImpl{val: "singleton"}
		},
	}

	provider := newProviderWith(desc)

	a := provider.GetRoutine(providerTyOf[iProviderFoo]())
	b := provider.GetRoutine(providerTyOf[iProviderFoo]())

	require.NotNil(t, a)
	require.NotNil(t, b)
	// 指针相同
	assert.Same(t, a.(*providerFooImpl), b.(*providerFooImpl))
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetRoutine_WhenTransient_ReturnsNewInstance
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetRoutine_WhenTransient_ReturnsNewInstance(t *testing.T) {
	counter := &factoryCounter{}
	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Transient,
		TyKey:    providerTyOf[iProviderFoo](),
		Factory: func(_ injection.IRoutineScope) any {
			counter.inc()
			return &providerFooImpl{val: "transient"}
		},
	}

	provider := newProviderWith(desc)

	a := provider.GetRoutine(providerTyOf[iProviderFoo]())
	b := provider.GetRoutine(providerTyOf[iProviderFoo]())

	require.NotNil(t, a)
	require.NotNil(t, b)
	// 应为两次独立实例
	assert.NotSame(t, a.(*providerFooImpl), b.(*providerFooImpl))
	// 工厂被调用了两次
	assert.EqualValues(t, 2, counter.total())
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetRoutine_WhenScoped_ReturnsSameInScope
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetRoutine_WhenScoped_ReturnsSameInScope(t *testing.T) {
	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Scoped,
		TyKey:    providerTyOf[iProviderFoo](),
		Factory: func(_ injection.IRoutineScope) any {
			return &providerFooImpl{val: "scoped"}
		},
	}

	provider := newProviderWith(desc)

	// 在同一 provider（同一 scope）中获取两次，应为同一实例
	a := provider.GetRoutine(providerTyOf[iProviderFoo]())
	b := provider.GetRoutine(providerTyOf[iProviderFoo]())
	require.NotNil(t, a)
	assert.Same(t, a.(*providerFooImpl), b.(*providerFooImpl))

	// 新 scope 中应为不同实例
	newScope := provider.CreateScope()
	c := newScope.GetProvider().GetRoutine(providerTyOf[iProviderFoo]())
	require.NotNil(t, c)
	assert.NotSame(t, a.(*providerFooImpl), c.(*providerFooImpl))
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_CreateScope_ReturnsNewScope
// ---------------------------------------------------------------------------

func TestRoutineProvider_CreateScope_ReturnsNewScope(t *testing.T) {
	provider := NewProvider(NewRoutineCollection(), nil)

	scope1 := provider.CreateScope()
	scope2 := provider.CreateScope()

	require.NotNil(t, scope1)
	require.NotNil(t, scope2)
	// 两个 scope 不是同一对象
	assert.NotSame(t, scope1.(*RoutineScope), scope2.(*RoutineScope))

	// Root 应都指向最顶层 provider 的 scope
	root1 := scope1.GetRoot()
	root2 := scope2.GetRoot()
	assert.Same(t, root1.(*RoutineScope), root2.(*RoutineScope))
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetRoutine_WhenNotRegistered_ReturnsNil
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetRoutine_WhenNotRegistered_ReturnsNil(t *testing.T) {
	provider := NewProvider(NewRoutineCollection(), nil)
	result := provider.GetRoutine(providerTyOf[iProviderFoo]())
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetKeyedRoutine_WhenKeyMatches_ReturnsInstance
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetKeyedRoutine_WhenKeyMatches_ReturnsInstance(t *testing.T) {
	key := "myKey"
	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		Key:      key,
		TyKey:    providerTyOf[iProviderFoo](),
		Factory: func(_ injection.IRoutineScope) any {
			return &providerFooImpl{val: "keyed"}
		},
	}

	col := NewRoutineCollection()
	col.AddDescriptor(desc)
	provider := NewProvider(col, nil)

	result := provider.GetKeyedRoutine(key, providerTyOf[iProviderFoo]())
	require.NotNil(t, result)
	assert.Equal(t, "keyed", result.(*providerFooImpl).val)

	// DefaultKey 找不到
	resultDefault := provider.GetRoutine(providerTyOf[iProviderFoo]())
	assert.Nil(t, resultDefault)
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_GetRootScope_ReturnsSameRoot
// ---------------------------------------------------------------------------

func TestRoutineProvider_GetRootScope_ReturnsSameRoot(t *testing.T) {
	provider := NewProvider(NewRoutineCollection(), nil)

	root1 := provider.GetRootScope()
	root2 := provider.GetRootScope()
	assert.Same(t, root1.(*RoutineScope), root2.(*RoutineScope))
}

// ---------------------------------------------------------------------------
// TestRoutineProvider_ConcurrentSingleton_InitializedOnce
// ---------------------------------------------------------------------------

func TestRoutineProvider_ConcurrentSingleton_InitializedOnce_ExpectAtomicInit(t *testing.T) {
	counter := &factoryCounter{}
	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    providerTyOf[iProviderBar](),
		Factory: func(_ injection.IRoutineScope) any {
			counter.inc()
			return &providerBarImpl{val: "bar"}
		},
	}

	col := NewRoutineCollection()
	col.AddDescriptor(desc)
	provider := NewProvider(col, nil)

	const goroutines = 100
	results := make([]any, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = provider.GetRoutine(providerTyOf[iProviderBar]())
		}(i)
	}

	wg.Wait()

	// 工厂只应被调用一次
	assert.EqualValues(t, 1, counter.total(), "singleton factory must be called exactly once")

	// 所有 goroutine 拿到的是同一实例
	first := results[0].(*providerBarImpl)
	for _, r := range results[1:] {
		assert.Same(t, first, r.(*providerBarImpl))
	}
}
