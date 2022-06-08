//go:build linux && debug
// +build linux,debug

package main

import (
	_ "github.com/ianlancetaylor/cgosymbolizer"
)

func ini() {
	runtime.SetCPUProfileRate(10_000)
}
