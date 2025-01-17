// armoryctl | https://github.com/usbarmory/armoryctl
//
// USB armory Mk II - hardware control tool
// Copyright (c) WithSecure Corporation
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build tamago && arm
// +build tamago,arm

package armoryctl

import (
	"fmt"

	"github.com/usbarmory/tamago/soc/nxp/imx6ul"
)

const I2CBus = 0

func init() {
	imx6ul.I2C1.Init()
}

func I2CRead(bus int, addr int, reg int16, size uint) (val []byte, err error) {
	if bus != I2CBus {
		return nil, fmt.Errorf("I2C bus must be set to %d", I2CBus)
	}

	if reg < 0 {
		return imx6ul.I2C1.Read(uint8(addr), 0, 0, int(size))
	} else {
		return imx6ul.I2C1.Read(uint8(addr), uint32(reg), 1, int(size))
	}
}

func I2CWrite(bus int, addr int, reg int16, val []byte) (err error) {
	if bus != I2CBus {
		return fmt.Errorf("I2C bus must be set to %d", I2CBus)
	}

	if reg < 0 {
		return imx6ul.I2C1.Write(val, uint8(addr), 0, 0)
	} else {
		return imx6ul.I2C1.Write(val, uint8(addr), uint32(reg), 1)
	}
}
