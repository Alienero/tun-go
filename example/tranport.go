package main

import (
	"log"
	"net"
	"sync"

	"github.com/FlexibleBroadband/tun-go"
)

// first start server tun server.
func main() {
	wg := sync.WaitGroup{}
	remoteAddr, err := net.ResolveUDPAddr("udp", "192.168.199.110:37989")
	if err != nil {
		panic(err)
	}
	localAddr, err := net.ResolveUDPAddr("udp", ":37988")
	if err != nil {
		panic(err)
	}
	// local tun interface read and write channel.
	rCh := make(chan []byte, 1024)
	wCh := make(chan []byte, 1024)

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		panic(err)
	}
	// read from udp conn, and write into tun.
	wg.Add(1)
	go func() {
		defer wg.Done()
		buff := make([]byte, 4096)
		for {
			n, err := conn.Read(buff)
			if err != nil {
				panic(err)
			}
			log.Println("tun<-conn:", n)
			// write into local tun interface channel.
			wCh <- buff[:n]
		}
	}()
	// read from local tun interface channel, and write into remote udp channel.
	wg.Add(1)
	go func() {
		wg.Done()
		for {
			select {
			case data := <-rCh:
				// write into udp conn.
				log.Println("tun->conn:", len(data))
				if _, err := conn.Write(data); err != nil {
					panic(err)
				}
			}
		}
	}()

	tuntap, err := tun.OpenTunTap(net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 2), net.IPv4(255, 255, 255, 0))
	if err != nil {
		panic(err)
	}
	defer tuntap.Close()
	// read data from tun into rCh channel.
	wg.Add(1)
	go func() {
		wg.Done()
		if err := tuntap.Read(rCh); err != nil {
			panic(err)
		}
	}()
	// write data into tun from wCh channel.
	wg.Add(1)
	go func() {
		wg.Done()
		if err := tuntap.Write(wCh); err != nil {
			panic(err)
		}
	}()
	wg.Wait()
}
