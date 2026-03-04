package node

import "sync"

// messagePool 用于复用 message 对象，减少热路径上的堆分配。
var messagePool = sync.Pool{
	New: func() any { return &message{} },
}

// rpcContextPool 用于复用 rpcContext 对象，减少热路径上的堆分配。
var rpcContextPool = sync.Pool{
	New: func() any { return &rpcContext{} },
}

// acquireMessage 从池中获取一个零值 message。
func acquireMessage() *message {
	return messagePool.Get().(*message)
}

// releaseMessage 将 message 归还到池中（必须清零所有字段以防数据泄漏）。
func releaseMessage(m *message) {
	m.ctx = nil
	m.nAddr = 0
	m.cb = nil
	m.timeout = 0
	m.src = 0
	m.dst = 0
	m.sess = 0
	m.trace = 0
	m.err = nil
	m.fName = ""
	m.args = nil
	m.data = nil
	messagePool.Put(m)
}
