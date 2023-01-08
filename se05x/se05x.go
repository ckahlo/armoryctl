// see NXP UM11225

package se05x

import (
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
				crc = (crc >> 1) ^ 0x8408
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
	pcb = frame[1]
	sz := uint(frame[2])

	if pcb&0xC0 == 0x80 {
		log.Printf("RDI2C: %X", frame)
	}

	out, _ = RDI2C(sz + 2) // +CRC

	out = out[0:sz]
	//count := res[0]
	//payload := res[:size]
	//data = res[1:size]
	//crc := res[size:]

	//log.Printf("RDI2C: %X", pkt)

	// XXX: if WTX, reply and read again
	// XXX: check CRC and request re-transmit if failed
	sz -= 2

	return
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
	} else {
		log.Printf("se05x: ATR ......: %v", err)
	}

	log.Printf("se05x: DEFAULT ..: %X", T1TX([]byte{0x00, 0xA4, 0x04, 0x00, 0x00}))
	log.Printf("se05x: CPLC .....: %X", T1TX([]byte{0x80, 0xCA, 0x9F, 0x7F, 0x00}))
	log.Printf("se05x: INFO .....: %X", T1TX([]byte{0x80, 0xCA, 0x00, 0xFE, 0x02, 0xDF, 0x28}))
	log.Printf("se05x: IOTSSD ...: %X", T1TX([]byte{0x00, 0xA4, 0x04, 0x00, 0x0B, 0xD2, 0x76, 0x00, 0x00, 0x85, 0x30, 0x4A, 0x43, 0x4F, 0x90, 0x03}))
	log.Printf("se05x: IOTAID ...: %X", T1TX([]byte{0x00, 0xA4, 0x04, 0x00, 0x10, 0xA0, 0x00, 0x00, 0x03, 0x96, 0x54, 0x53, 0x00, 0x00, 0x00, 0x01, 0x03, 0x00, 0x00, 0x00, 0x00}))
	log.Printf("se05x: VERSION ..: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x20, 0x00}))
	log.Printf("se05x: VERSION-X : %X", T1TX([]byte{0x80, 0x04, 0x00, 0x21, 0x00}))
	log.Printf("se05x: TIMESTAMP : %X", T1TX([]byte{0x80, 0x04, 0x00, 0x3D, 0x00}))
	log.Printf("se05x: FREE MEM .: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x22, 0x03, 0x41, 0x01, 0x01}))
	log.Printf("se05x: RANDOM ...: %X", T1TX([]byte{0x80, 0x04, 0x00, 0x49, 0x04, 0x41, 0x02, 0x00, 0x40}))

	return
}
