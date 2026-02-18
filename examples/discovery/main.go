package main

import (
	"github.com/gmbytes/snow/core/host"
	"github.com/gmbytes/snow/core/host/builder"
	"github.com/gmbytes/snow/routines/ignore_input"
	"github.com/gmbytes/snow/routines/node"
)

func main() {
	// 1. 创建服务发现实例并预注册服务地址
	//    生产环境中这些地址通常由注册中心（etcd/Consul）自动维护
	disc := NewMapDiscovery()
	disc.Register("Pong", "127.0.0.1", 8000)

	// 2. 构建应用
	b := builder.NewDefaultBuilder()
	host.AddHostedRoutine[*ignore_input.IgnoreInput](b)

	host.AddOption[*node.Option](b, "Node")
	host.AddOptionFactory[*node.Option](b, func() *node.Option {
		return &node.Option{
			BootName: "MyNode",
			LocalIP:  "127.0.0.1",
			Nodes: map[string]*node.ElementOption{
				"MyNode": {
					Port:     8000,
					HttpPort: 8080,
					Services: []string{"Ping", "Pong"},
				},
			},
		}
	})

	// 3. 注册节点，传入 ServiceDiscovery
	//    - CreateProxy("Pong") 会优先通过 disc.Resolve("Pong") 获取地址
	//    - Node.Stop() 会自动调用 disc.Deregister() 注销服务
	//    - GET /health 端点始终可用，返回 200(ok) 或 503(draining)
	node.AddNode(b, func() *node.RegisterOption {
		return &node.RegisterOption{
			ServiceRegisterInfos: []*node.ServiceRegisterInfo{
				node.CheckedServiceRegisterInfoName[ping](1, "Ping"),
				node.CheckedServiceRegisterInfoName[pong](2, "Pong"),
			},
			ServiceDiscovery: disc,
		}
	})

	host.Run(b.Build())
}
