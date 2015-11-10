// +build linux darwin

package tun

import (
	"fmt"
	"os"
)

func write(fd *os.File, ch chan []byte) error {
	for {
		select {
		case data := <-ch:
			if _, err := fd.Write(data); err != nil {
				return err
			}
		}
	}
}

func read(fd *os.File, mtu int, ch chan []byte) error {
	buf := make([]byte, mtu)
	for {
		n, err := fd.Read(buf)
		if err != nil {
			return err
		}
		// check length.
		totalLen := 0
		switch buf[0] & 0xf0 {
		case 0x40:
			totalLen = 256*int(buf[2]) + int(buf[3])
		case 0x60:
			totalLen = 256*int(buf[4]) + int(buf[5]) + IPv6_HEADER_LENGTH
		}
		if totalLen != n {
			return fmt.Errorf("read n(%v)!=total(%v)", n, totalLen)
		}
		send := make([]byte, totalLen)
		copy(send, buf)
		ch <- send
	}
}

func close(fd *os.File) error {
	return fd.Close()
}
