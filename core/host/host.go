package host

import (
	"reflect"

	"github.com/gmbytes/snow/core/injection"
)

type IHost interface {
	IHostedRoutine

	GetRoutineProvider() injection.IRoutineProvider
}

func GetRoutine[T any](provider injection.IRoutineProvider) T {
	ty := reflect.TypeOf((*T)(nil)).Elem()
	instance := provider.GetRoutine(ty)
	if instance == nil {
		var zero T
		return zero
	}
	if result, ok := instance.(T); ok {
		return result
	}
	// 类型不匹配时 panic，因为这是编程错误
	panic("GetRoutine: type assertion failed")
}
