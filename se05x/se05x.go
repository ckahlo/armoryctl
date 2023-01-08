// see NXP UM11225

package se05x

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"

	armoryctl "github.com/usbarmory/armoryctl/internal"
)

var (
	I2CBus     = 0
	I2CAddress = 0x48 // mk2.SE050_ADDR
)

const (
	NAD  uint8 = 0x5A
	IFSC int   = 254
)

// https://www.nxp.com/docs/en/application-note/AN13013.pdf
var SE05xType = map[uint16]string{
	0xA1F4: "SE050_C2 (OM-SE050ARD)",
	0xA200: "SE050_C1 (03_XX)",
	0xA201: "SE050_C2 (03_XX)",
	0xA202: "SE050_B1 (03_XX)",
	0xA203: "SE050_B2 (03_XX)",
	0xA204: "SE050_A1 (03_XX)",
	0xA205: "SE050_A2 (03_XX)",
	0xA564: "SE051_C2 (06_00)",
	0xA565: "SE051_A2 (06_00)",
	0xA739: "SE051_W2 (07_02)",
	0xA77E: "SE050_F2 (03_XX, 2021)",
	0xA8FA: "SE051_C2 (07_02)",
	0xA920: "SE051_A2 (07_02)",
	0xA921: "SE050_E2 (07_02, OM-SE050ARD-E)",
	0xA92A: "SE050_F2 (03_XX, 2022 / OM-SE050ARD-F)",
}

var (
	maxTries int = 50
	apduCtr  uint8
)

func WRI2C(data []byte) (err error) {
	for i := 0; i < maxTries; i++ {
		if err = armoryctl.I2CWrite(I2CBus, I2CAddress, -1, data); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	return
}

func RDI2C(le uint) (data []byte, err error) {
	for i := 0; i < maxTries; i++ {
		if data, err = armoryctl.I2CRead(I2CBus, I2CAddress, -1, le); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	return
}

/* */
func CCITTCRC16(data []byte, crc uint16) uint16 {
	for i := 0; i < len(data); i++ {
		crc ^= uint16(data[i])
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408 // reversed, non-reciprocal
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func I2CTX(pcb uint8, in []byte) (out []byte, err error) {
	if pcb == 0xCF { // reset ADPU counter on reset / ATR
		apduCtr = 0
	}

	if len(in) >= IFSC { // XXX: use ATR IFSC to check for max frame size
		return nil, fmt.Errorf("data field exceeds IFSC %v/%v", len(in), IFSC)
	}

	frame := append([]byte{NAD, pcb, uint8(len(in))}, in...)
	crc := ^CCITTCRC16(frame, 0xFFFF) // watch out for negation and byte order of CRC
	frame = append(frame, []byte{byte(crc & 0xFF), byte(crc >> 8)}...)

	//log.Printf("WRI2C: %X", pkt)
	if err = WRI2C(frame); err != nil {
		return
	}

	if frame, err = RDI2C(3); err != nil {
		return
	}
	//log.Printf("RDI2C: %X", pkt)

	// CHECK [0] NAD = 0xA5 & PCB chain/number
	// XXX: if WTX, reply and read again
	pcb = frame[1]
	sz := uint(frame[2])

	if pcb&0xC0 == 0x80 {
		log.Printf("RDI2C: %X", frame)
	}

	out, _ = RDI2C(sz + 2) // +CRC

	if 0xF0B8 != CCITTCRC16(out, CCITTCRC16(frame, 0xFFFF)) {
		return nil, fmt.Errorf("CRC Error: %04X", out[sz:])
	}

	return out[:sz], err
}

func T1TX(apdu []byte) []byte {
	var chain uint8 = 0
	if read, err := I2CTX(((apduCtr&1)<<6)|(chain<<5), apdu); err == nil {
		apduCtr++
		return read
	} else {
		log.Printf("I2CTX error: %v", err)
	}

	return nil
}

func selectAID(aid []byte) []byte {
	return T1TX(append([]byte{0x00, 0xA4, 0x04, 0x00, uint8(len(aid))}, aid...))
}

func nonnull(val, err any) any {
	if err != nil {
		return err
	}
	return val
}

func Info() (res string, err error) {

	//armoryctl.Logger = &log.Logger{}
	log.Printf("se05x info")

	log.Printf("se05x: SYNC .....: %v", nonnull(I2CTX(0xC0, nil)))
	log.Printf("se05x: RESET ....: %v", nonnull(I2CTX(0xC6, nil)))
	//log.Printf("se05x: IFSC .....: %v", nonnull(I2CTX(0xC1, []byte{0x20})))

	if atr, err := I2CTX(0xCF, nil); err == nil {
		log.Printf("se05x: ATR ......: %X", atr)
		log.Printf("se05x:             /protocol version : %04X   /vendor ID .......: %04X", atr[0], atr[1:6])
		log.Printf("se05x:             /BWT .............: %04X   /IFSC ............: %04X", atr[7:9], atr[9:11])
		log.Printf("se05x:             /PLID (2 = I2C) ..: %04X   /I2C clock max ...: %04X   /I2C config ......: %04X", atr[11], atr[13:15], atr[15])
		log.Printf("se05x:             /historical bytes : %04X (%v)", atr[25:35], string(atr[25:35]))
	} else {
		log.Printf("se05x: ATR ......: %v", err)
	}

	buf := selectAID(nil)
	log.Printf("se05x: DEFAULT ..: %X", buf)
	log.Printf("se05x:             /AID              : %X", buf[4:4+buf[3]])

	buf = T1TX([]byte{0x80, 0xCA, 0x9F, 0x7F, 0x00})
	log.Printf("se05x: CPLC .....: %X", buf)
	log.Printf("se05x:             /IC Fabricator ...: %04X   /IC Type .........: %04X   /OS ID ...........: %04X", buf[3:5], buf[5:7], buf[7:9])
	log.Printf("se05x:             /OS release date..: %04X   /OS releave level : %04X   /IC fab date .....: %04X", buf[9:11], buf[11:13], buf[13:15])
	log.Printf("se05x:             /IC serial no.....: %04X   /IC batch ID .....: %04X", buf[15:19], buf[19:21])

	buf = T1TX([]byte{0x80, 0xCA, 0x00, 0xFE, 0x02, 0xDF, 0x28})
	log.Printf("se05x: INFO .....: %X", buf) // https://www.nxp.com/docs/en/application-note/AN13013.pdf
	log.Printf("se05x:             /OEF ID ..........: %04X   /Platform build ID: %v   /Type: %v", buf[9:11], string(buf[31:47]), SE05xType[binary.BigEndian.Uint16(buf[9:11])])

	log.Printf("se05x: IOTSSD ...: %X", selectAID([]byte{0xD2, 0x76, 0x00, 0x00, 0x85, 0x30, 0x4A, 0x43, 0x4F, 0x90, 0x03}))
	log.Printf("se05x: IOTAID ...: %X", selectAID([]byte{0xA0, 0x00, 0x00, 0x03, 0x96, 0x54, 0x53, 0x00, 0x00, 0x00, 0x01, 0x03, 0x00, 0x00, 0x00, 0x00}))
	log.Printf("se05x: VERSION ..: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x20, 0x00}))
	log.Printf("se05x: VERSION-X : %X", T1TX([]byte{0x80, 0x04, 0x00, 0x21, 0x00}))
	log.Printf("se05x: TIMESTAMP : %X", T1TX([]byte{0x80, 0x04, 0x00, 0x3D, 0x00}))
	log.Printf("se05x: FREE MEM .: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x22, 0x03, 0x41, 0x01, 0x01}))
	log.Printf("se05x: RANDOM ...: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x49, 0x04, 0x41, 0x02, 0x00, 0x40}))

	return
}
