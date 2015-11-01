package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	TAPWIN32_MAX_REG_SIZE = 256
	TUNTAP_COMPONENT_ID   = "tap0901"
	ADAPTER_KEY           = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
)

var (
	TAP_IOCTL_GET_MTU          = tap_control_code(3, 0)
	TAP_IOCTL_SET_MEDIA_STATUS = tap_control_code(6, 0)
	TAP_IOCTL_CONFIG_TUN       = tap_control_code(10, 0)

	TUN_IPv4_ADDRESS = net.IPv4(10, 1, 0, 1)          // < The IPv4 address of the TUN interface.
	TUN_IPv4_NETWORK = net.IPv4(10, 1, 0, 0)          // < The IPv4 address of the TUN interface's network.
	TUN_IPv4_NETMASK = net.IPv4Mask(255, 255, 255, 0) // < The IPv4 netmask of the TUN interface.
)

func main() {
	taptun, mtu, err := openTunTap()
	if err != nil {
		panic(err)
	}
	fmt.Println(taptun, mtu)
}

func openTunTap() (syscall.Handle, int, error) {
	id, err := getTuntapComponentId()
	if err != nil {
		return 0, 0, err
	}
	device_path := fmt.Sprintf(`\\.\Global\%s.tap`, id)
	name := syscall.StringToUTF16(device_path)
	tuntap, err := syscall.CreateFile(
		&name[0],
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_SYSTEM|syscall.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		return 0, 0, err
	}
	fmt.Println("tuntap", tuntap)
	var returnLen uint32
	configTunParam := append(TUN_IPv4_ADDRESS, TUN_IPv4_NETWORK...)
	configTunParam = append(configTunParam, TUN_IPv4_NETMASK...)
	configTunParam = chrs(configTunParam)
	if err = syscall.DeviceIoControl(
		tuntap,
		TAP_IOCTL_CONFIG_TUN,
		&configTunParam[0],
		uint32(len(configTunParam)),
		&configTunParam[0],
		uint32(len(configTunParam)),
		&returnLen,
		nil); err != nil {
		return 0, 0, err
	}

	// get MTU
	var umtu = make([]byte, 4)
	if err = syscall.DeviceIoControl(
		tuntap,
		TAP_IOCTL_GET_MTU,
		nil,
		0,
		&umtu[0],
		uint32(len(umtu)),
		&returnLen,
		nil); err != nil {
		return 0, 0, err
	}
	mtu := binary.LittleEndian.Uint32(umtu)
	fmt.Println(mtu, returnLen)

	// set connect.
	inBuffer := []byte("\x01\x00\x00\x00")
	if err = syscall.DeviceIoControl(
		tuntap,
		TAP_IOCTL_SET_MEDIA_STATUS,
		&inBuffer[0],
		uint32(len(inBuffer)),
		&inBuffer[0],
		uint32(len(inBuffer)),
		&returnLen,
		nil); err != nil {
		return 0, 0, err
	}
	return tuntap, int(mtu), nil
}

func getTuntapComponentId() (string, error) {
	adapters, err := registry.OpenKey(registry.LOCAL_MACHINE, ADAPTER_KEY, registry.READ)
	if err != nil {
		return "", err
	}
	var i uint32
	for ; i < 1000; i++ {
		var name_length uint32 = TAPWIN32_MAX_REG_SIZE
		buf := make([]uint16, name_length)
		if err = syscall.RegEnumKeyEx(
			syscall.Handle(adapters),
			i,
			&buf[0],
			&name_length,
			nil,
			nil,
			nil,
			nil); err != nil {
			return "", err
		}
		key_name := syscall.UTF16ToString(buf[:])
		// fmt.Println("key_name", key_name)
		adapter, err := registry.OpenKey(adapters, key_name, registry.READ)
		if err != nil {
			return "", err
		}
		name := syscall.StringToUTF16("ComponentId")
		name2 := syscall.StringToUTF16("NetCfgInstanceId")
		var valtype uint32
		var component_id = make([]byte, TAPWIN32_MAX_REG_SIZE)
		// var pbuf = (*byte)(unsafe.Pointer(&component_id[0]))
		var componentLen = uint32(len(component_id))
		if err = syscall.RegQueryValueEx(
			syscall.Handle(adapter),
			&name[0],
			nil,
			&valtype,
			&component_id[0],
			&componentLen); err != nil {
			return "", err
		}

		if unicodeTostring(component_id) == TUNTAP_COMPONENT_ID {
			var valtype uint32
			var netCfgInstanceId = make([]byte, TAPWIN32_MAX_REG_SIZE)
			var netCfgInstanceIdLen = uint32(len(netCfgInstanceId))
			if err = syscall.RegQueryValueEx(
				syscall.Handle(adapter),
				&name2[0],
				nil,
				&valtype,
				&netCfgInstanceId[0],
				&netCfgInstanceIdLen); err != nil {
				return "", err
			}
			fmt.Println("last:", unicodeTostring(netCfgInstanceId))
			return unicodeTostring(netCfgInstanceId), nil
		}
	}
	return "", errors.New("not found component id")
}

func ReadChannel(taptun syscall.Handle, mtu int, ch chan []byte) (err error) {
	overlappedRx := syscall.Overlapped{}
	var hevent windows.Handle
	hevent, err = windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return
	}
	overlappedRx.HEvent = syscall.Handle(hevent)
	buf := make([]byte, mtu)
	var l uint32
	for {
		if err = syscall.ReadFile(taptun, buf, &l, &overlappedRx); err != nil {
			return
		}
		if _, err = syscall.WaitForSingleObject(overlappedRx.HEvent, syscall.INFINITE); err != nil {
			return
		}
		overlappedRx.Offset += l
		// send data into channel.
		ch <- buf[:l]
	}
}

func WriteFromChannel(taptun syscall.Handle, ch chan []byte) (err error) {
	overlappedRx := syscall.Overlapped{}
	var hevent windows.Handle
	hevent, err = windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return
	}
	overlappedRx.HEvent = syscall.Handle(hevent)
	for {
		select {
		case data := <-ch:
			var l uint32
			if err = syscall.WriteFile(taptun, data, &l, &overlappedRx); err != nil {
				return nil
			}
			if _, err = syscall.WaitForSingleObject(overlappedRx.HEvent, syscall.INFINITE); err != nil {
				return
			}
			overlappedRx.Offset += uint32(len(data))
			fmt.Println("data len:", len(data), "write:", l)
		}
	}
}

func unicodeTostring(src []byte) string {
	dst := make([]byte, 0)
	for _, ch := range src {
		if ch != byte(0) {
			dst = append(dst, ch)
		}
	}
	return string(dst)
}

func ctl_code(device_type, function, method, access uint32) uint32 {
	return (device_type << 16) | (access << 14) | (function << 2) | method
}

func tap_control_code(request, method uint32) uint32 {
	return ctl_code(34, request, method, 0)
}

func chrs(src []byte) []byte {
	var s string
	for _, c := range src {
		s += string(c)
	}
	return []byte(s)
}
