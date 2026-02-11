package math

import "cmp"

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

func Clamp[T cmp.Ordered](v T, minV T, maxV T) T {
	return max(min(v, maxV), minV)
}

func Abs[T Numeric](a T) T {
	if a < 0 {
		return -a
	}
	return a
}
