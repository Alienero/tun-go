package tun

import (
	// "encoding/binary"
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
)

type tun struct {
	mtu         int
	device_path string
	fd          syscall.Handle
}

func OpenTunTap(addr net.IP, network net.IP, mask net.IP) (Tun, error) {
	t := new(tun)
	id, err := getTuntapComponentId()
	if err != nil {
		return nil, err
	}
	t.device_path = fmt.Sprintf(`\\.\Global\%s.tap`, id)
	name := syscall.StringToUTF16(t.device_path)
	tuntap, err := syscall.CreateFile(
		&name[0],
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_SYSTEM|syscall.FILE_FLAG_OVERLAPPED,
		0)
	if err != nil {
		fmt.Println("here")
		return nil, err
	}
	var returnLen uint32
    var configTunParam []byte = append(addr.To4(), network.To4()...)
    configTunParam = append(configTunParam, mask.To4()...)
    fmt.Println(configTunParam)
    //configTunParam = []byte{10, 0, 0, 1, 10, 0, 0, 0, 255, 255, 255, 0}
	if err = syscall.DeviceIoControl(
		tuntap,
		TAP_IOCTL_CONFIG_TUN,
		&configTunParam[0],
		uint32(len(configTunParam)),
		&configTunParam[0],
		uint32(len(configTunParam)),
		&returnLen,
		nil); err != nil {
		fmt.Println("here2")
		return nil, err
	}

	// get MTU
	// var umtu = make([]byte, 4)
	// if err = syscall.DeviceIoControl(
	// 	tuntap,
	// 	TAP_IOCTL_GET_MTU,
	// 	nil,
	// 	0,
	// 	&umtu[0],
	// 	uint32(len(umtu)),
	// 	&returnLen,
	// 	nil); err != nil {
	// 	fmt.Println("here3")
	// 	return nil, err
	// }
	// mtu := binary.LittleEndian.Uint32(umtu)
	mtu := 1500

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
		return nil, err
	}
	t.fd = tuntap
	t.mtu = int(mtu)
	return t, nil
}

func getTuntapComponentId() (string, error) {
    k, err := registry.OpenKey(registry.LOCAL_MACHINE,
        ADAPTER_KEY,
        registry.ENUMERATE_SUB_KEYS | registry.QUERY_VALUE)
    if err != nil {
        return "", err
    }
    defer k.Close()

    names, err := k.ReadSubKeyNames(-1)
    if err != nil {
        return "", err
    }

    for _, name := range names {
        n, _ := matchKey(k, name, TUNTAP_COMPONENT_ID)
        if n != "" {
            return n, nil
        }
    }
    return "", fmt.Errorf("Not Found")

}
func matchKey(zones registry.Key, kname string, componentId string) (string, error) {
    k, err := registry.OpenKey(zones, kname, registry.READ)
    if err != nil {
        return "", err
    }
    defer k.Close()

    cId, _, err := k.GetStringValue("ComponentId")
    if cId == componentId {
        netCfgInstanceId, _, err := k.GetStringValue("NetCfgInstanceId")
        if err != nil {
            return "", err
        }
        return netCfgInstanceId, nil

    }
    return "", fmt.Errorf("ComponentId != componentId")
}

func (t *tun) Read(ch chan []byte) (err error) {
	overlappedRx := syscall.Overlapped{}
	var hevent windows.Handle
	hevent, err = windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return
	}
	overlappedRx.HEvent = syscall.Handle(hevent)
	buf := make([]byte, t.mtu)
	var l uint32
	for {
		if err := syscall.ReadFile(t.fd, buf, &l, &overlappedRx); err != nil {
		}
		if _, err := syscall.WaitForSingleObject(overlappedRx.HEvent, syscall.INFINITE); err != nil {
			fmt.Println(err)
		}
        //overlappedRx.Offset += l
        totalLen := overlappedRx.InternalHigh
        /*switch buf[0] & 0xf0 {
        case 0x40:
            totalLen = 256 * int(buf[2]) + int(buf[3])
        case 0x60:
            continue
            totalLen = 256 * int(buf[4]) + int(buf[5]) + IPv6_HEADER_LENGTH
        }*/
        //fmt.Println("read data", buf[:totalLen])
        send := make([]byte, totalLen)
        copy(send, buf)
        ch <- send
	}
}

func (t *tun) Write(ch chan []byte) (err error) {
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
			syscall.WriteFile(t.fd, data, &l, &overlappedRx)
			syscall.WaitForSingleObject(overlappedRx.HEvent, syscall.INFINITE)
			overlappedRx.Offset += uint32(len(data))
		}
	}
}

func (t *tun) Close() error {
	return syscall.Close(t.fd)
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
