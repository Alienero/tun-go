package tun

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

type tuntap struct {
	mtu        int
	actualName string
	fd         *os.File
}

func OpenTunTap(addr net.IP, mask net.IPMask) (Tun, error) {
	t := &tuntap{
		mtu: 1500,
	}
	dynamicOpened := false
	for i := 0; i < 16; i++ {
		tunName := fmt.Sprintf("/dev/tun%d", i)
		t.actualName = fmt.Sprintf("tun%d", i)
		fd, err := os.OpenFile(tunName, os.O_RDWR, 0)
		if err == nil {
			t.fd = fd
			dynamicOpened = true
			break
		}
		log.Printf("[WARN] Failed to open TUN/TAP device '%s': %v", t.actualName, err)
	}
	if !dynamicOpened {
		return nil, errors.New("cannot allocate TUN/TAP device dynamically.")
	}

	log.Printf("[INFO] TUN/TAP device %s opened.", t.actualName)
	if err := t.setupAddress(addr, mask); err != nil {
		return nil, err
	}
	return t, nil
}

func (tun *tuntap) setupAddress(addr net.IP, mask net.IPMask) error {
	cmd := exec.Command("/sbin/ifconfig", tun.actualName, "delete")
	_ = cmd.Run()
	log.Printf("[INFO] NOTE: Tried to delete pre-existing TUN/TAP instance -- no problem if failed.")

	cmd = exec.Command("/sbin/ifconfig", tun.actualName,
		addr.String(), addr.String(), "netmask", mask.String(), "mtu", "1500", "up")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Mac OS X ifconfig failed: %v: %s", err, output)
	}
	return nil
}

func (tun *tuntap) ReadFromChannel(ch chan []byte) error {
	for {
		select {
		case data := <-ch:
			if _, err := tun.fd.Write(data); err != nil {
				return err
			}
		}
	}
}

func (tun *tuntap) WriteIntoChannel(ch chan []byte) error {
	buf := make([]byte, tun.mtu)
	for {
		n, err := tun.fd.Read(buf)
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

func (tun *tuntap) Close() error {
	return tun.fd.Close()
}
