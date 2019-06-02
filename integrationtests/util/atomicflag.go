package util

import (
	"sync/atomic"
)

type AtomicFlag int32

func (af *AtomicFlag) Get() bool {
	return atomic.LoadInt32((*int32)(af)) == 1
}

func (af *AtomicFlag) Set(val bool) {
	if val {
		atomic.StoreInt32((*int32)(af), 1)
	} else {
		atomic.StoreInt32((*int32)(af), 0)
	}
}

func (af *AtomicFlag) SetIf(old, new bool) (set bool) {
	var o, n int32
	if old {
		o = 1
	}
	if new {
		n = 1
	}
	return atomic.CompareAndSwapInt32((*int32)(af), o, n)
}
