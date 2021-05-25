// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	_ "net/http/pprof"
	"os"
	"runtime"

	utils "github.com/filechain/filechain/btcwallet/maindep"
)

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Work around defer not working after os.Exit.
	if err := utils.WalletMain(nil); err != nil {
		os.Exit(1)
	}
}
