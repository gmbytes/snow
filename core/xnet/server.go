package xnet

// Server 通用网络服务接口。
type Server interface {
	Start() error
	Stop()
}
