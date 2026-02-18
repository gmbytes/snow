package main

import (
	"fmt"
	"sync"

	"github.com/gmbytes/snow/core/logging/slog"
	"github.com/gmbytes/snow/routines/node"
)

// MapDiscovery 基于内存映射的服务发现实现，演示 IServiceDiscovery 接口用法。
// 生产环境可替换为 etcd / Consul / ZooKeeper 等实现。
type MapDiscovery struct {
	mu       sync.RWMutex
	registry map[string]node.INodeAddr // serviceName -> nodeAddr
}

func NewMapDiscovery() *MapDiscovery {
	return &MapDiscovery{
		registry: make(map[string]node.INodeAddr),
	}
}

// Register 注册服务地址（模拟注册中心写入）。
func (d *MapDiscovery) Register(serviceName string, host string, port int) error {
	addr, err := node.NewNodeAddr(host, port)
	if err != nil {
		return fmt.Errorf("invalid address %s:%d: %w", host, port, err)
	}
	d.mu.Lock()
	d.registry[serviceName] = addr
	d.mu.Unlock()
	slog.Infof("[Discovery] registered %s -> %s:%d", serviceName, host, port)
	return nil
}

// Resolve 实现 node.IServiceDiscovery。
func (d *MapDiscovery) Resolve(serviceName string) (node.INodeAddr, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if addr, ok := d.registry[serviceName]; ok {
		slog.Infof("[Discovery] resolved %s -> %s", serviceName, addr.String())
		return addr, nil
	}
	return nil, fmt.Errorf("service %q not found in discovery", serviceName)
}

// Deregister 实现 node.IServiceDiscovery，停机时由框架自动调用。
func (d *MapDiscovery) Deregister(nodeAddr node.INodeAddr, services []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, name := range services {
		delete(d.registry, name)
		slog.Infof("[Discovery] deregistered %s (node=%s)", name, nodeAddr.String())
	}
}
