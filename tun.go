package main

import (
	"fmt"
	"syscall"
	"time"
	// "unsafe"

	// "golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	TAPWIN32_MAX_REG_SIZE = 256
	TUNTAP_COMPONENT_ID   = "tap0901"
)

func main() {
	getTuntapComponentId()
	time.Sleep(10 * time.Second)
}

var ADAPTER_KEY = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`

func getTuntapComponentId() {
	adapters, err := registry.OpenKey(registry.LOCAL_MACHINE, ADAPTER_KEY, registry.READ)
	if err != nil {
		panic(err)
	}
	fmt.Println(adapters)
	var i uint32
	for ; i < 1000; i++ {
		var name_length uint32 = TAPWIN32_MAX_REG_SIZE
		buf := make([]uint16, name_length)
		if err = syscall.RegEnumKeyEx(syscall.Handle(adapters), i, &buf[0], &name_length, nil, nil, nil, nil); err != nil {
			panic(err)
		}
		key_name := syscall.UTF16ToString(buf[:])
		// fmt.Println("key_name", key_name)
		adapter, err := registry.OpenKey(adapters, key_name, registry.READ)
		if err != nil {
			fmt.Println(err)
			continue
		}
		name := syscall.StringToUTF16("ComponentId")
		name2 := syscall.StringToUTF16("NetCfgInstanceId")
		var valtype uint32
		var component_id = make([]byte, TAPWIN32_MAX_REG_SIZE)
		// var pbuf = (*byte)(unsafe.Pointer(&component_id[0]))
		var componentLen = uint32(len(component_id))
		if err = syscall.RegQueryValueEx(syscall.Handle(adapter), &name[0], nil, &valtype, &component_id[0], &componentLen); err != nil {
			panic(err)
		}

		if unicodeTostring(component_id) == TUNTAP_COMPONENT_ID {
			var valtype uint32
			var netCfgInstanceId = make([]byte, TAPWIN32_MAX_REG_SIZE)
			var netCfgInstanceIdLen = uint32(len(netCfgInstanceId))
			if err = syscall.RegQueryValueEx(syscall.Handle(adapter), &name2[0], nil, &valtype, &netCfgInstanceId[0], &netCfgInstanceIdLen); err != nil {
				panic(err)
			}
			fmt.Println("last:", unicodeTostring(netCfgInstanceId))
			return
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
