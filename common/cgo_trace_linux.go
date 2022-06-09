//go:build linux && debug
// +build linux,debug

package common

import (
	"runtime"

	//_ "github.com/ianlancetaylor/cgosymbolizer"
	_ "github.com/benesch/cgosymbolizer"
)

func ini() {
	runtime.SetCPUProfileRate(10_000)
}
