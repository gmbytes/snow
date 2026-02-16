package version

import "math"

type DBVersion int64

// GetAppCurrentDBVersion 获取 APP 当前数据版本号
func GetAppCurrentDBVersion() DBVersion {
	return DBVersion20241011
}

const (
	DBVersionPrimitive DBVersion = iota          // 第一个版本
	DBVersion20241011                            // 注释描述版本升级内容
	DBVersionMax       DBVersion = math.MaxInt64 // 最大版本号
)
