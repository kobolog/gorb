package main

import (
	"fmt"
	"gnl2go/gnl2go"
)

func main() {
	fmt.Println("hi there")
	ipvs := new(gnl2go.IpvsClient)
	err := ipvs.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = ipvs.Flush()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ipvs.Exit()
	p, _ := ipvs.GetPools()
	fmt.Printf("%#v\n", p)
	ipvs.AddService("2a02::33", 80, uint16(gnl2go.ToProtoNum("tcp")), "wlc")
	ipvs.AddService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")), "wrr")
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.11", uint16(gnl2go.ToProtoNum("tcp")), 10)
	//10 - syscall.AF_INET6, 2 - syscall.AF_INET
	ipvs.AddFWMService(1, "wrr", 10)
	err = ipvs.AddFWMService(1, "wrr", 10)
	//This is expected error coz we already have this service
	if err != nil {
		fmt.Println(err)
	}
	ipvs.AddFWMService(2, "rr", 2)
	p, _ = ipvs.GetPools()
	fmt.Printf("%#v\n", p)
	ipvs.DelService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")))
	ipvs.DelService("2a02::33", 80, uint16(gnl2go.ToProtoNum("tcp")))
	ipvs.DelFWMService(1, 10)
	ipvs.DelFWMService(2, 2)
	ipvs.AddService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")), "wrr")
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.11", uint16(gnl2go.ToProtoNum("tcp")), 10)
	ipvs.AddDestPort("192.168.1.1", 80, "127.0.0.11", 8080, uint16(gnl2go.ToProtoNum("tcp")), 10, gnl2go.IPVS_MASQUERADING)
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.12", uint16(gnl2go.ToProtoNum("tcp")), 10)
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.13", uint16(gnl2go.ToProtoNum("tcp")), 10)
	ipvs.UpdateDest("192.168.1.1", 80, "127.0.0.13", uint16(gnl2go.ToProtoNum("tcp")), 20)
	ipvs.DelDest("192.168.1.1", 80, "127.0.0.12", uint16(gnl2go.ToProtoNum("tcp")))
	ipvs.AddFWMService(1, "wrr", 10)
	ipvs.AddFWMDestFWD(1, "fc00:1::12", 10, 0, 10, gnl2go.IPVS_MASQUERADING)
	err = ipvs.AddFWMDest(10, "fc00:1::12", 10, 0, 10)
	//Expected: there is no service with fwmark 2
	if err != nil {
		fmt.Println(err)
	}
	ipvs.AddFWMDest(1, "fc00:2::12", 10, 0, 20)
	ipvs.AddFWMDest(1, "fc00:2:3::12", 10, 0, 30)
	ipvs.UpdateFWMDest(1, "fc00:2:3::12", 10, 0, 33)
	ipvs.DelFWMDest(1, "fc00:2::12", 10, 0)
	fmt.Println("done")
}
