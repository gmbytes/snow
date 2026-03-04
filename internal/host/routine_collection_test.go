package internal

import (
	"reflect"
	"testing"

	"github.com/gmbytes/snow/pkg/injection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 辅助：测试用接口/结构
type iTestService interface{ Do() }
type testServiceImpl struct{}

func (t *testServiceImpl) Do() {}

type iAnotherService interface{ Run() }
type anotherServiceImpl struct{}

func (a *anotherServiceImpl) Run() {}

func tyOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_AddDescriptor_WhenSingleton
// ---------------------------------------------------------------------------

func TestRoutineCollection_AddDescriptor_WhenSingleton_ExpectDescriptorStored(t *testing.T) {
	col := NewRoutineCollection()

	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    tyOf[iTestService](),
		TyImpl:   tyOf[testServiceImpl](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}

	col.AddDescriptor(desc)

	got := col.GetDescriptor(tyOf[iTestService]())
	require.NotNil(t, got)
	assert.Equal(t, injection.Singleton, got.Lifetime)
	assert.Equal(t, tyOf[iTestService](), got.TyKey)
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_AddDescriptor_WhenKeyNil_SetsDefaultKey
// ---------------------------------------------------------------------------

func TestRoutineCollection_AddDescriptor_WhenKeyNil_SetsDefaultKey(t *testing.T) {
	col := NewRoutineCollection()

	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		Key:      nil, // 故意为 nil
		TyKey:    tyOf[iTestService](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}

	col.AddDescriptor(desc)

	// Key 应已被设为 DefaultKey
	assert.Equal(t, injection.DefaultKey, desc.Key)
	// 通过 GetDescriptor（内部用 DefaultKey）能查到
	got := col.GetDescriptor(tyOf[iTestService]())
	require.NotNil(t, got)
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_GetDescriptors_ReturnsAll
// ---------------------------------------------------------------------------

func TestRoutineCollection_GetDescriptors_ReturnsAll_ExpectBothDescriptors(t *testing.T) {
	col := NewRoutineCollection()

	desc1 := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		TyKey:    tyOf[iTestService](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}
	desc2 := &injection.RoutineDescriptor{
		Lifetime: injection.Transient,
		TyKey:    tyOf[iAnotherService](),
		Factory:  func(_ injection.IRoutineScope) any { return &anotherServiceImpl{} },
	}

	col.AddDescriptor(desc1)
	col.AddDescriptor(desc2)

	all := col.GetDescriptors()
	assert.Len(t, all, 2)

	types := make(map[reflect.Type]bool)
	for _, d := range all {
		types[d.TyKey] = true
	}
	assert.True(t, types[tyOf[iTestService]()])
	assert.True(t, types[tyOf[iAnotherService]()])
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_GetDescriptor_ByType
// ---------------------------------------------------------------------------

func TestRoutineCollection_GetDescriptor_ByType_ExpectCorrectDescriptor(t *testing.T) {
	col := NewRoutineCollection()

	desc := &injection.RoutineDescriptor{
		Lifetime: injection.Scoped,
		TyKey:    tyOf[iTestService](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}
	col.AddDescriptor(desc)

	got := col.GetDescriptor(tyOf[iTestService]())
	require.NotNil(t, got)
	assert.Equal(t, injection.Scoped, got.Lifetime)

	// 查不存在的类型应返回 nil
	notFound := col.GetDescriptor(tyOf[iAnotherService]())
	assert.Nil(t, notFound)
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_GetKeyedDescriptor
// ---------------------------------------------------------------------------

func TestRoutineCollection_GetKeyedDescriptor_WhenDifferentKeys_ExpectIsolation(t *testing.T) {
	col := NewRoutineCollection()

	keyA := "keyA"
	keyB := "keyB"

	descA := &injection.RoutineDescriptor{
		Lifetime: injection.Singleton,
		Key:      keyA,
		TyKey:    tyOf[iTestService](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}
	descB := &injection.RoutineDescriptor{
		Lifetime: injection.Transient,
		Key:      keyB,
		TyKey:    tyOf[iTestService](),
		Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
	}

	col.AddDescriptor(descA)
	col.AddDescriptor(descB)

	gotA := col.GetKeyedDescriptor(keyA, tyOf[iTestService]())
	gotB := col.GetKeyedDescriptor(keyB, tyOf[iTestService]())

	require.NotNil(t, gotA)
	require.NotNil(t, gotB)
	assert.Equal(t, injection.Singleton, gotA.Lifetime)
	assert.Equal(t, injection.Transient, gotB.Lifetime)
}

func TestRoutineCollection_GetKeyedDescriptor_WhenKeyNotExist_ExpectNil(t *testing.T) {
	col := NewRoutineCollection()

	got := col.GetKeyedDescriptor("nonexistent", tyOf[iTestService]())
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// TestRoutineCollection_Concurrent_AddDescriptor（并发安全）
// ---------------------------------------------------------------------------

func TestRoutineCollection_Concurrent_AddDescriptor_ExpectNoPanic(t *testing.T) {
	col := NewRoutineCollection()
	done := make(chan struct{})
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			desc := &injection.RoutineDescriptor{
				Lifetime: injection.Singleton,
				Key:      idx, // 不同 key 避免写冲突
				TyKey:    tyOf[iTestService](),
				Factory:  func(_ injection.IRoutineScope) any { return &testServiceImpl{} },
			}
			col.AddDescriptor(desc)
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}

	all := col.GetDescriptors()
	assert.Len(t, all, goroutines)
}
