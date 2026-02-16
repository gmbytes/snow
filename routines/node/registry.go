package node

import (
	"reflect"
	"sync"

	"github.com/gmbytes/snow/core/host"
)

// registeredService 记录通过 Register 注册的服务
type registeredService struct {
	info  *ServiceRegisterInfo
	setup func(b host.IBuilder) // 可选的构建期回调（如绑定 Option 配置）
}

var (
	registryMu         sync.Mutex
	registeredServices []*registeredService
	autoKind           int32 = 1
)

// Register 自动注册服务，在服务包的 init() 中调用。
//
// Kind 自动递增分配，Name 为服务名。
// 可选 setup 回调在 RegisterService 时执行，用于绑定配置等构建期操作。
//
// 用法：
//
//	func init() {
//	    node.Register[MyService, *MyService]("MyService")
//	}
//
// 带配置绑定：
//
//	func init() {
//	    node.Register[MyService, *MyService]("MyService", func(b host.IBuilder) {
//	        host.AddOption[MyConfig](b, "MyService")
//	    })
//	}
func Register[T any, U consService[T]](name string, setup ...func(host.IBuilder)) {
	registryMu.Lock()
	defer registryMu.Unlock()

	s := &registeredService{
		info: &ServiceRegisterInfo{
			Kind: autoKind,
			Name: name,
			Type: reflect.PointerTo(reflect.TypeFor[T]()),
		},
	}
	if len(setup) > 0 {
		s.setup = setup[0]
	}
	registeredServices = append(registeredServices, s)
	autoKind++
}

// GetRegisteredService 获取所有自动注册的服务名称列表。
// 可直接用于 ElementOption.Services。
func GetRegisteredService() []string {
	registryMu.Lock()
	defer registryMu.Unlock()

	names := make([]string, len(registeredServices))
	for i, s := range registeredServices {
		names[i] = s.info.Name
	}
	return names
}

// RegisterService 自动注册所有服务到节点。
// 替代手动 AddNode + 逐个 CheckedServiceRegisterInfoName 的方式。
// 可选 opts 用于设置 RegisterOption 的其他字段（如 Preprocessor 等）。
func RegisterService(b host.IBuilder, opts ...func(*RegisterOption)) {
	registryMu.Lock()
	defer registryMu.Unlock()

	// 执行各服务的 setup 回调（如绑定 Option 配置）
	for _, s := range registeredServices {
		if s.setup != nil {
			s.setup(b)
		}
	}

	// 收集服务注册信息
	infos := make([]*ServiceRegisterInfo, len(registeredServices))
	for i, s := range registeredServices {
		infos[i] = s.info
	}

	AddNode(b, func() *RegisterOption {
		opt := &RegisterOption{
			ServiceRegisterInfos: infos,
		}
		for _, fn := range opts {
			fn(opt)
		}
		return opt
	})
}
