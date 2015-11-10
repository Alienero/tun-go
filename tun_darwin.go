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

func OpenTunTap(addr net.IP, network net.IP, mask net.IP) (Tun, error) {
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
	if err := t.setupAddress(addr, network, mask); err != nil {
		return nil, err
	}
	return t, nil
}

func (tun *tuntap) setupAddress(addr net.IP, network net.IP, mask net.IP) error {
	cmd := exec.Command("/sbin/ifconfig", tun.actualName, "delete")
	_ = cmd.Run()
	log.Printf("[INFO] NOTE: Tried to delete pre-existing TUN/TAP instance -- no problem if failed.")

	cmd = exec.Command("/sbin/ifconfig", tun.actualName,
		addr.String(), network.String(), "netmask", mask.String(), "mtu", "1500", "up")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Mac OS X ifconfig failed: %v: %s", err, output)
	}
	return nil
}

func (tun *tuntap) Write(ch chan []byte) error {
	return write(tun.fd, ch)
}

func (tun *tuntap) Read(ch chan []byte) error {
	return read(tun.fd, tun.mtu, ch)
}

func (tun *tuntap) Close() error {
	return close(tun.fd)
}
