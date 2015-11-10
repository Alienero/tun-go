package tun

import (
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	IFF_NO_PI = 0x10
	IFF_TUN   = 0x01
	IFF_TAP   = 0x02
	TUNSETIFF = 0x400454CA
)

type tuntap struct {
	mtu        int
	actualName string
	fd         *os.File
}

func OpenTunTap(addr net.IP, network net.IP, mask net.IP) (Tun, error) {
	tun := new(tuntap)
	deviceFile := "/dev/net/tun"
	fd, err := os.OpenFile(deviceFile, os.O_RDWR, 0)
	if err != nil {
		log.Fatalf("[CRIT] Note: Cannot open TUN/TAP dev %s: %v", deviceFile, err)
	}
	tun.fd = fd

	ifr := make([]byte, 18)
	ifr[17] = IFF_NO_PI
	ifr[16] = IFF_TUN
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(tun.fd.Fd()), uintptr(TUNSETIFF),
		uintptr(unsafe.Pointer(&ifr[0])))
	if errno != 0 {
		log.Fatalf("[CRIT] Cannot ioctl TUNSETIFF: %v", errno)
	}

	tun.actualName = string(ifr)
	tun.actualName = tun.actualName[:strings.Index(tun.actualName, "\000")]
	log.Printf("[INFO] TUN/TAP device %s opened.", tun.actualName)
	if err := tun.setupAddress(addr.String(), mask.String()); err != nil {
		return nil, err
	} else {
		return tun, nil
	}
}

func (tun *tuntap) setupAddress(addr, mask string) error {
	cmd := exec.Command("ifconfig", tun.actualName, addr,
		"netmask", mask, "mtu", "1500")
	log.Printf("[DEBG] ifconfig command: %v", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err != nil {
		log.Printf("[EROR] Linux ifconfig failed: %v.", err)
		return err
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
