package main

import (
	"context"

	"github.com/gmbytes/snow/pkg/host"
	"github.com/gmbytes/snow/pkg/host/builder"
	"github.com/gmbytes/snow/pkg/routines/ignore_input"
	"github.com/gmbytes/snow/pkg/routines/node"
)

func main() {
	b := builder.NewDefaultBuilder()
	host.AddHostedRoutine[*ignore_input.IgnoreInput](b)

	host.AddOption[*node.Option](b, "Node")
	host.AddOptionFactory[*node.Option](b, func() *node.Option {
		return &node.Option{
			BootName: "MyNode",
			LocalIP:  "127.0.0.1",
			Nodes: map[string]*node.ElementOption{
				"MyNode": {
					Services: []string{"Ping", "Pong"},
				},
			},
		}
	})
	node.AddNode(b, func() *node.RegisterOption {
		return &node.RegisterOption{
			ServiceRegisterInfos: []*node.ServiceRegisterInfo{
				node.CheckedServiceRegisterInfoName[ping](1, "Ping"),
				node.CheckedServiceRegisterInfoName[pong](2, "Pong"),
			},
		}
	})

	host.Run(b.Build(), context.Background())
}
