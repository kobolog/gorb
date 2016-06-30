package main

import (
	"fmt"
	"gnl2go/gnl2go"
)

/* we can ran this as binary for unit tests (for example build it on
   on one machine(mac or win) and run on other(linux), probably
   later will convert into ipvs_test.go */
func main() {
	fmt.Println("Start Test/Example Run")
	ipvs := new(gnl2go.IpvsClient)
	err := ipvs.Init()
	if err != nil {
		fmt.Printf("Cant initialize client, erro is %#v\n", err)
		return
	}
	err = ipvs.Flush()
	if err != nil {
		fmt.Printf("Error while running Flush method %#v\n", err)
		return
	}
	defer ipvs.Exit()
	p, err := ipvs.GetPools()
	if err != nil {
		fmt.Printf("Error while running GetPools method %#v\n", err)
		return
	}
	if len(p) != 0 {
		fmt.Printf("Flush method havent cleared all the data\n")
		return
	}
	noIPv6 := false
	//Testing IPv6 AddService
	err = ipvs.AddService("2a02::33", 80, uint16(gnl2go.ToProtoNum("tcp")), "wlc")
	if err != nil {
		fmt.Printf(`
			Error while adding IPv6 Service w/ AddService: %#v
			could be because of lack ipv6 support in compiled ipvs
			(default in rasberian)
			`, err)
		noIPv6 = true
	}

	//Testing IPv4 AddService
	err = ipvs.AddService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")), "wrr")
	if err != nil {
		fmt.Printf("cant add ipv4 service w/ AddService; err is : %#v\n", err)
		return
	}

	//Testing AddDest for IPv4
	err = ipvs.AddDest("192.168.1.1", 80, "127.0.0.11", uint16(gnl2go.ToProtoNum("tcp")), 10)
	if err != nil {
		fmt.Printf("cant add ipv4 with AddDest; err is : %#v\n", err)
		return
	}

	//10 - syscall.AF_INET6, 2 - syscall.AF_INET
	//Testing AddFWMService for IPv6
	if !noIPv6 {
		err = ipvs.AddFWMService(1, "wrr", 10)
		if err != nil {
			fmt.Printf("cant add ipv6 service w/ AddFWMService; err is : %#v\n", err)
			return
		}
		err = ipvs.AddFWMService(1, "wrr", 10)
		//This is expected error because we already have this service
		if err != nil {
			fmt.Printf("Expected error because we adding existing service : %#v\n", err)
		}
	}

	//Testing AddFWMService for IPv4
	err = ipvs.AddFWMService(2, "rr", 2)
	if err != nil {
		fmt.Printf("cant add ipv4 service w/ AddFWMService; err is : %#v\n", err)
		return
	}
	p, _ = ipvs.GetPools()
	if !noIPv6 {
		if len(p) != 4 {
			fmt.Printf("Something wrong, len of pool not equal",
				" to 4 but %v instead\n", len(p))
			return
		}
	} else {
		if len(p) != 2 {
			fmt.Printf("Something wrong, len of pool not equal",
				" to 2 but %v instead\n", len(p))
			return
		}
	}
	//Testing deleitation of the ipv4 service
	err = ipvs.DelService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")))
	if err != nil {
		fmt.Printf("error while running DelService for ipv4: %#v\n", err)
		return
	}
	if !noIPv6 {
		err = ipvs.DelService("2a02::33", 80, uint16(gnl2go.ToProtoNum("tcp")))
		if err != nil {
			fmt.Printf("error while deleting ipv6 service: %#v\n", err)
			return
		}
		err = ipvs.DelFWMService(1, 10)
		if err != nil {
			fmt.Printf("error while deleting ipv6 fwmark service: %#v\n", err)
			return
		}
	}
	err = ipvs.DelFWMService(2, 2)
	if err != nil {
		fmt.Printf("error while delete ipv4 fwmark service: %#v\n", err)
		return
	}
	/* AddService and AddDest already covered */
	ipvs.AddService("192.168.1.1", 80, uint16(gnl2go.ToProtoNum("tcp")), "wrr")
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.11", uint16(gnl2go.ToProtoNum("tcp")), 10)
	/* Testing AddDestPort */
	err = ipvs.AddDestPort("192.168.1.1", 80, "127.0.0.11", 8080,
		uint16(gnl2go.ToProtoNum("tcp")),
		10, gnl2go.IPVS_MASQUERADING)
	if err != nil {
		fmt.Printf("error while running AddDestPort for ipv4: %#v\n", err)
		return
	}
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.12", uint16(gnl2go.ToProtoNum("tcp")), 10)
	ipvs.AddDest("192.168.1.1", 80, "127.0.0.13", uint16(gnl2go.ToProtoNum("tcp")), 10)
	/* Testing Update Dest */
	err = ipvs.UpdateDest("192.168.1.1", 80, "127.0.0.13", uint16(gnl2go.ToProtoNum("tcp")), 20)
	if err != nil {
		fmt.Printf("error while running UpdateDest for ipv4: %#v\n", err)
		return
	}
	/* Testing DelDest */
	err = ipvs.DelDest("192.168.1.1", 80, "127.0.0.12", uint16(gnl2go.ToProtoNum("tcp")))
	if err != nil {
		fmt.Printf("error while running DelDest for ipv4: %#v\n", err)
	}
	if !noIPv6 {
		ipvs.AddFWMService(1, "wrr", 10)
		err = ipvs.AddFWMDestFWD(1, "fc00:1::12", 10, 0, 10, gnl2go.IPVS_MASQUERADING)
		if err != nil {
			fmt.Printf("error while running AddFWMDestFWD for ipv6: %#v\n", err)
			return
		}
		err = ipvs.AddFWMDest(10, "fc00:1::12", 10, 0, 10)
		//Expected: there is no service with fwmark 10
		if err != nil {
			fmt.Printf("expected error because of lack fwmark 10: %#v\n", err)
		}

		ipvs.AddFWMDest(1, "fc00:2::12", 10, 0, 20)
		ipvs.AddFWMDest(1, "fc00:2:3::12", 10, 0, 30)
		/* Testing UpdateFWMDest */
		err = ipvs.UpdateFWMDest(1, "fc00:2:3::12", 10, 0, 33)
		if err != nil {
			fmt.Printf("error while running UpdateFWMDest : %#v\n", err)
			return
		}
		/* Testing DelFWMDest */
		err = ipvs.DelFWMDest(1, "fc00:2::12", 10, 0)
		if err != nil {
			fmt.Printf("error while running UpdateFWMDest: %#v\n", err)
			return
		}
	}
	/* Testing IPv4 Service w/ Flags */
	err = ipvs.AddServiceWithFlags("192.168.1.22", 50100,
		uint16(gnl2go.ToProtoNum("udp")), "sh",
		gnl2go.BIN_IP_VS_SVC_F_SCHED_SH_FALLBACK)
	if err != nil {
		fmt.Printf(`
		cant add ipv4 service w/ AddServiceWithFlags; err is : %#v\n`, err)
		return
	}
	err = ipvs.AddDestPort("192.168.1.22", 50100, "192.168.1.2",
		8080, uint16(gnl2go.ToProtoNum("udp")), 10, gnl2go.IPVS_MASQUERADING)
	if err != nil {
		fmt.Printf("cant add 1st dest to service w/  sched flags: %#v\n", err)
		return
	}
	ipvs.AddDestPort("192.168.1.22", 50100, "192.168.1.2",
		8081, uint16(gnl2go.ToProtoNum("udp")), 10, gnl2go.IPVS_MASQUERADING)
	if err != nil {
		fmt.Printf("cant add 2nd dest to service w/  sched flags: %#v\n", err)
		return
	}
	/* Testing IPv4 Service w/ Flags (thru helper routine) */
	err = ipvs.AddServiceWithFlags("192.168.1.22", 50101,
		uint16(gnl2go.ToProtoNum("udp")), "sh",
		gnl2go.U32ToBinFlags(
			gnl2go.IP_VS_SVC_F_SCHED_SH_FALLBACK|gnl2go.IP_VS_SVC_F_SCHED_SH_PORT))
	if err != nil {
		fmt.Printf("error while adding service w/ flags and helper: %#v\n", err)
		return
	}
	fmt.Println("done")
}
