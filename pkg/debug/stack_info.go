package debug

import (
	"runtime"
)

const stackBufSize = 4096

func StackInfo() string {
	buf := make([]byte, stackBufSize)
	buf = buf[:runtime.Stack(buf, false)]
	return string(buf)
}
