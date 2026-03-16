package handler

import (
	"reflect"
	"unsafe"

	"github.com/gmbytes/snow/core/logging"
)

var _ logging.ILogHandler = (*RootHandler)(nil)

type RootHandler struct {
	proxy logging.ILogHandler
}

func NewRootHandler(proxy logging.ILogHandler) *RootHandler {
	return &RootHandler{
		proxy: proxy,
	}
}

func (ss *RootHandler) Log(data *logging.LogData) {
	if ss.proxy != nil {
		ss.proxy.Log(data)
	}
}

func (ss *RootHandler) WrapToContainer(ty reflect.Type) any {
	if ty == nil {
		return nil
	}
	
	elemTy := ty.Elem()
	if elemTy.Kind() != reflect.Struct {
		return nil
	}
	
	if elemTy.NumField() == 0 {
		return nil
	}
	
	keyTy := elemTy.Field(0).Type

	instanceValue := reflect.New(elemTy)
	instance := instanceValue.Interface()

	fieldValue := reflect.ValueOf(ss)

	field := reflect.NewAt(keyTy, unsafe.Pointer(instanceValue.Elem().Field(0).UnsafeAddr())).Elem()
	field.Set(fieldValue)
	return instance
}
