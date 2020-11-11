// Copyright (c) Edgeless Systems GmbH.
// Licensed under the MIT License.

// +build !enclave

package main

import "github.com/edgelesssys/marblerun/marble/premain"

func init() {
	if err := premain.PreMainMock(); err != nil {
		panic(err)
	}
}
