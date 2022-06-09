package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cmd/integration/commands"
)

func main() {
	f, err := os.Create("cpu.pprof")
	if err != nil {
		fmt.Println("could not create CPU profile: ", err)
		os.Exit(1)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Println("could not start CPU profile: ", err)
		os.Exit(1)
	}
	defer pprof.StopCPUProfile()

	rootCmd := commands.RootCommand()
	ctx, _ := common.RootContext()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
