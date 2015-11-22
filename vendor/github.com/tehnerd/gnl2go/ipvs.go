package gnl2go

/*
This package implements routines to work with linux LVS, so we would be able
to work with LVS nativly, instead of using Exec(ipvsadm).
It is expecting for end user to work with this routines:

ipvs := new(IpvsClient)
err := ipvs.Init() - to init netlink socket etc

err := ipvs.Flush() - to flush lvs table

pools,err := ipvs.GetPools() - to check which services and dest has been configured

err := ipvs.AddService(vip_string,port_uint16,protocol_uint16,scheduler_string) - add new service,
	which will be described by it's address (lvs also support service by fwmark (from iptables), check bellow)
	type of service(ipv4 or ipv6) will be deduced from it's vip address

err := ipvs.DelService(vip_string,port_uint16,protocol_uint16) - to delete service

err := ipvs.AddFWMService(fwmark_uint32,sched_string,af_uint16) - add fwmark service, we must also
	provide the type of service (af; must be syscall.AF_INET for ipv4 or syscall.AF_INET6 for ipv6)

err := ipvs.DelFWMService(fwmark_uint32,af_uint16) - delete fwmark service

err := ipvs.AddDest(vip_string,port_uint16,rip_string,protocol_uint16,weight_int32) - add destination rip to vip. port on vip and rip is the same
	fwding methond - tunneling

err := ipvs.AddDestPort(vip_string,vport_uint16,rip_string,rport_uint16,protocol_uint16,weight_int32,fwd_uint32) - add destination rip to vip.
	port on vip and rip could be different. fwding method could be any supported (for example IPVS_MASQUERADING)

err := ipvs.UpdateDest(vip_string,port_uint16,rip_string,protocol_uint16,weight_int32) - change description of real server(for example
	change it's weight)

err := ipvs.UpdateDestPort(vip_string,vport_uint16,rip_string,rport_uint16, protocol_uint16,weight_int32,fwd_uint32) - same as above
	but with custom ports on real and fwd method

err := ipvs.DelDest(vip_string,port_uint16,rip_string,protocol_uint16)

err := ipvs.DelDestPort(vip_string,vport_uint16,rip_string,rport_uint16, protocol_uint16)


err := ipvs.AddFWMDest(fwmark_uint32,rip_string,vaf_uint16,port_uint16,weight_int32) - add destination to fwmark bassed service,
	vaf - fwmark's service address family.

err := ipvs.AddFWMDestFWD(fwmark_uint32,rip_string,vaf_uint16,port_uint16,weight_int32,fwd_uint32) - add destination to fwmark bassed service,
	vaf - fwmark's service address family. fwd - forwarding method (tunneling or nat/masquerading)

err := ipvs.UpdateFWMDest(fwmark_uint32,rip_string,vaf_uint16,port_uint16,weight_int32)

err := ipvs.UpdateFWMDestFWD(fwmark_uint32,rip_string,vaf_uint16,port_uint16,weight_int32,fwd_uint32)

err := DelFWMDest(fwmark_uint32,rip_string,vaf_uint16,port_uint16)

ipvs.Exit() - to close NL socket

*/

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
)

const (
	IPVS_MASQUERADING = 0
	IPVS_TUNNELING    = 2
)

var (
	IpvsStatsAttrList = CreateAttrListDefinition("IpvsStatsAttrList",
		[]AttrTuple{
			AttrTuple{Name: "CONNS", Type: "U32Type"},
			AttrTuple{Name: "INPKTS", Type: "U32Type"},
			AttrTuple{Name: "OUTPKTS", Type: "U32Type"},
			AttrTuple{Name: "INBYTES", Type: "U64Type"},
			AttrTuple{Name: "OUTBYTES", Type: "U64Type"},
			AttrTuple{Name: "CPS", Type: "U32Type"},
			AttrTuple{Name: "INPPS", Type: "U32Type"},
			AttrTuple{Name: "OUTPPS", Type: "U32Type"},
			AttrTuple{Name: "INBPS", Type: "U32Type"},
			AttrTuple{Name: "OUTBPS", Type: "U32Type"},
		})

	IpvsServiceAttrList = CreateAttrListDefinition("IpvsServiceAttrList",
		[]AttrTuple{
			AttrTuple{Name: "AF", Type: "U16Type"},
			AttrTuple{Name: "PROTOCOL", Type: "U16Type"},
			AttrTuple{Name: "ADDR", Type: "BinaryType"},
			AttrTuple{Name: "PORT", Type: "Net16Type"},
			AttrTuple{Name: "FWMARK", Type: "U32Type"},
			AttrTuple{Name: "SCHED_NAME", Type: "NulStringType"},
			AttrTuple{Name: "FLAGS", Type: "BinaryType"},
			AttrTuple{Name: "TIMEOUT", Type: "U32Type"},
			AttrTuple{Name: "NETMASK", Type: "U32Type"},
			AttrTuple{Name: "STATS", Type: "IpvsStatsAttrList"},
			AttrTuple{Name: "PE_NAME", Type: "NulStringType"},
		})

	IpvsDestAttrList = CreateAttrListDefinition("IpvsDestAttrList",
		[]AttrTuple{
			AttrTuple{Name: "ADDR", Type: "BinaryType"},
			AttrTuple{Name: "PORT", Type: "Net16Type"},
			AttrTuple{Name: "FWD_METHOD", Type: "U32Type"},
			AttrTuple{Name: "WEIGHT", Type: "I32Type"},
			AttrTuple{Name: "U_THRESH", Type: "U32Type"},
			AttrTuple{Name: "L_THRESH", Type: "U32Type"},
			AttrTuple{Name: "ACTIVE_CONNS", Type: "U32Type"},
			AttrTuple{Name: "INACT_CONNS", Type: "U32Type"},
			AttrTuple{Name: "PERSIST_CONNS", Type: "U32Type"},
			AttrTuple{Name: "STATS", Type: "IpvsStatsAttrList"},
			AttrTuple{Name: "ADDR_FAMILY", Type: "U16Type"},
		})

	IpvsDaemonAttrList = CreateAttrListDefinition("IpvsDaemonAttrList",
		[]AttrTuple{
			AttrTuple{Name: "STATE", Type: "U32Type"},
			AttrTuple{Name: "MCAST_IFN", Type: "NulStringType"},
			AttrTuple{Name: "SYNC_ID", Type: "U32Type"},
		})

	IpvsInfoAttrList = CreateAttrListDefinition("IpvsInfoAttrList",
		[]AttrTuple{
			AttrTuple{Name: "VERSION", Type: "U32Type"},
			AttrTuple{Name: "CONN_TAB_SIZE", Type: "U32Type"},
		})

	IpvsCmdAttrList = CreateAttrListDefinition("IpvsCmdAttrList",
		[]AttrTuple{
			AttrTuple{Name: "SERVICE", Type: "IpvsServiceAttrList"},
			AttrTuple{Name: "DEST", Type: "IpvsDestAttrList"},
			AttrTuple{Name: "DAEMON", Type: "IpvsDaemonAttrList"},
			AttrTuple{Name: "TIMEOUT_TCP", Type: "U32Type"},
			AttrTuple{Name: "TIMEOUT_TCP_FIN", Type: "U32Type"},
			AttrTuple{Name: "TIMEOUT_UDP", Type: "U32Type"},
		})

	IpvsMessageInitList = []AttrListTuple{
		AttrListTuple{Name: "NEW_SERVICE", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "SET_SERVICE", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "DEL_SERVICE", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "GET_SERVICE", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "NEW_DEST", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "SET_DEST", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "DEL_DEST", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "GET_DEST", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "NEW_DAEMON", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "DEL_DAEMON", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "GET_DAEMON", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "SET_CONFIG", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "GET_CONFIG", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "SET_INFO", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "GET_INFO", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "ZERO", AttrList: CreateAttrListType(IpvsCmdAttrList)},
		AttrListTuple{Name: "FLUSH", AttrList: CreateAttrListType(IpvsCmdAttrList)},
	}
)

func validateIp(ip string) bool {
	for _, c := range ip {
		if c == ':' {
			_, err := IPv6StringToAddr(ip)
			if err != nil {
				return false
			}
			return true
		}
	}
	_, err := IPv4ToUint32(ip)
	if err != nil {
		return false
	}
	return true
}

func toAFUnion(ip string) (uint16, []byte, error) {
	buf := new(bytes.Buffer)
	for _, c := range ip {
		if c == ':' {
			addr, _ := IPv6StringToAddr(ip)
			err := binary.Write(buf, binary.BigEndian, addr)
			if err != nil {
				return 0, nil, err
			}
			encAddr := buf.Bytes()
			if len(encAddr) != 16 {
				return 0, nil, fmt.Errorf("length not equal to 16\n")
			}
			return syscall.AF_INET6, encAddr, nil
		}
	}
	addr, err := IPv4ToUint32(ip)
	if err != nil {
		return 0, nil, err
	}
	err = binary.Write(buf, binary.BigEndian, addr)
	if err != nil {
		return 0, nil, err
	}
	encAddr := buf.Bytes()
	for len(encAddr) != 16 {
		encAddr = append(encAddr, byte(0))
	}
	return syscall.AF_INET, encAddr, nil
}

func fromAFUnion(af uint16, addr []byte) (string, error) {
	if af == syscall.AF_INET6 {
		var v6addr IPv6Addr
		err := binary.Read(bytes.NewReader(addr), binary.BigEndian, &v6addr)
		if err != nil {
			return "", fmt.Errorf("cant decode ipv6 addr from net repr:%v\n", err)
		}
		addrStr := IPv6AddrToString(v6addr)
		return addrStr, nil
	}
	var v4addr uint32
	//we leftpadded addr to len 16 above,so our v4 addr in addr[12:]
	err := binary.Read(bytes.NewReader(addr[:4]), binary.BigEndian, &v4addr)
	if err != nil {
		return "", fmt.Errorf("cant decode ipv4 addr from net repr:%v\n", err)
	}
	addrStr := Uint32IPv4ToString(v4addr)
	return addrStr, nil
}

func ToProtoNum(proto NulStringType) U16Type {
	p := string(proto)
	switch strings.ToLower(p) {
	case "tcp":
		return U16Type(syscall.IPPROTO_TCP)
	case "udp":
		return U16Type(syscall.IPPROTO_UDP)
	}
	return U16Type(0)
}

func FromProtoNum(pnum U16Type) NulStringType {
	switch uint16(pnum) {
	case syscall.IPPROTO_TCP:
		return NulStringType("TCP")
	case syscall.IPPROTO_UDP:
		return NulStringType("UDP")
	}
	return NulStringType("UNKNOWN")
}

type Dest struct {
	IP     string
	Weight int32
	Port   uint16
	AF     uint16
}

func (d *Dest) IsEqual(od *Dest) bool {
	return d.IP == od.IP && d.Weight == od.Weight && d.Port == od.Port
}

func (d *Dest) InitFromAttrList(list map[string]SerDes) error {
	//lots of casts from interface w/o checks; so we are going to panic if something goes wrong
	af, ok := list["ADDR_FAMILY"].(*U16Type)
	if !ok {
		//OLD kernel (3.18-), which doesnt support addr_family in dest definition
		dAF := U16Type(d.AF)
		af = &dAF
	} else {
		d.AF = uint16(*af)
	}
	addr, ok := list["ADDR"].(*BinaryType)
	if !ok {
		return fmt.Errorf("no dst ADDR in attr list: %#v\n", list)
	}
	ip, err := fromAFUnion(uint16(*af), []byte(*addr))
	if err != nil {
		return err
	}
	d.IP = ip
	w, ok := list["WEIGHT"].(*I32Type)
	if !ok {
		return fmt.Errorf("no dst WEIGHT in attr list: %#v\n", list)
	}
	d.Weight = int32(*w)
	p, ok := list["PORT"].(*Net16Type)
	if !ok {
		return fmt.Errorf("no dst PORT in attr list: %#v\n", list)
	}
	d.Port = uint16(*p)
	return nil
}

type Service struct {
	Proto  uint16
	VIP    string
	Port   uint16
	Sched  string
	FWMark uint32
	AF     uint16
}

func (s *Service) IsEqual(os Service) bool {
	return s.Proto == os.Proto && s.VIP == os.VIP &&
		s.Port == os.Port && s.Sched == os.Sched && s.FWMark == os.FWMark
}

func (s *Service) InitFromAttrList(list map[string]SerDes) error {
	if _, exists := list["ADDR"]; exists {
		af := list["AF"].(*U16Type)
		s.AF = uint16(*af)
		addr := list["ADDR"].(*BinaryType)
		vip, err := fromAFUnion(uint16(*af), []byte(*addr))
		if err != nil {
			return err
		}
		s.VIP = vip
		proto := list["PROTOCOL"].(*U16Type)
		s.Proto = uint16(*proto)
		p := list["PORT"].(*Net16Type)
		s.Port = uint16(*p)
	} else {
		fw := list["FWMARK"].(*U32Type)
		s.FWMark = uint32(*fw)

	}
	sched := list["SCHED_NAME"].(*NulStringType)
	s.Sched = string(*sched)
	return nil
}

type Pool struct {
	Service Service
	Dests   []Dest
}

func (p *Pool) InitFromAttrList(list map[string]SerDes) {
	//TODO(tehnerd):...
}

type IpvsClient struct {
	Sock NLSocket
	mt   *MessageType
}

func (ipvs *IpvsClient) Init() error {
	LookupTypeOnStartup(IpvsMessageInitList, "IPVS")
	err := ipvs.Sock.Init()
	if err != nil {
		return err
	}
	ipvs.mt = Family2MT[MT2Family["IPVS"]]
	return nil
}

func (ipvs *IpvsClient) Flush() error {
	msg, err := ipvs.mt.InitGNLMessageStr("FLUSH", ACK_REQUEST)
	if err != nil {
		return err
	}
	err = ipvs.Sock.Execute(msg)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) GetPools() ([]Pool, error) {
	var pools []Pool
	msg, err := ipvs.mt.InitGNLMessageStr("GET_SERVICE", MATCH_ROOT_REQUEST)
	if err != nil {
		return nil, err
	}
	resps, err := ipvs.Sock.Query(msg)
	if err != nil {
		return nil, err
	}
	for _, resp := range resps {
		var pool Pool
		svcAttrList := resp.GetAttrList("SERVICE")
		pool.Service.InitFromAttrList(svcAttrList.(*AttrListType).Amap)
		destReq, err := ipvs.mt.InitGNLMessageStr("GET_DEST", MATCH_ROOT_REQUEST)
		if err != nil {
			return nil, err
		}
		destReq.AttrMap["SERVICE"] = svcAttrList.(*AttrListType)
		destResps, err := ipvs.Sock.Query(destReq)
		if err != nil {
			return nil, err
		}
		for _, destResp := range destResps {
			var d Dest
			dstAttrList := destResp.GetAttrList("DEST")
			d.AF = pool.Service.AF
			if dstAttrList != nil {
				d.InitFromAttrList(dstAttrList.(*AttrListType).Amap)
				pool.Dests = append(pool.Dests, d)
			}
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

func (ipvs *IpvsClient) modifyService(method string, vip string,
	port uint16, protocol uint16, amap map[string]SerDes) error {
	af, addr, err := toAFUnion(vip)
	if err != nil {
		return err
	}
	//1<<32-1
	netmask := uint32(4294967295)
	if af == syscall.AF_INET6 {
		netmask = 128
	}
	msg, err := ipvs.mt.InitGNLMessageStr(method, ACK_REQUEST)
	if err != nil {
		return err
	}
	AF := U16Type(af)
	Port := Net16Type(port)
	Netmask := U32Type(netmask)
	Addr := BinaryType(addr)
	Proto := U16Type(protocol)
	Flags := BinaryType([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	atl, _ := ATLName2ATL["IpvsServiceAttrList"]
	sattr := CreateAttrListType(atl)
	sattr.Amap["AF"] = &AF
	sattr.Amap["PORT"] = &Port
	sattr.Amap["PROTOCOL"] = &Proto
	sattr.Amap["ADDR"] = &Addr
	sattr.Amap["NETMASK"] = &Netmask
	sattr.Amap["FLAGS"] = &Flags
	for k, v := range amap {
		sattr.Amap[k] = v
	}
	msg.AttrMap["SERVICE"] = &sattr
	err = ipvs.Sock.Execute(msg)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) AddService(vip string,
	port uint16, protocol uint16, sched string) error {
	paramsMap := make(map[string]SerDes)
	Sched := NulStringType(sched)
	Timeout := U32Type(0)
	paramsMap["SCHED_NAME"] = &Sched
	paramsMap["TIMEOUT"] = &Timeout
	err := ipvs.modifyService("NEW_SERVICE", vip, port,
		protocol, paramsMap)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) DelService(vip string,
	port uint16, protocol uint16) error {
	err := ipvs.modifyService("DEL_SERVICE", vip, port,
		protocol, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) modifyFWMService(method string, fwmark uint32,
	af uint16, amap map[string]SerDes) error {
	AF := U16Type(af)
	FWMark := U32Type(fwmark)
	netmask := uint32(4294967295)
	if af == syscall.AF_INET6 {
		netmask = 128
	}
	msg, err := ipvs.mt.InitGNLMessageStr(method, ACK_REQUEST)
	if err != nil {
		return err
	}
	Netmask := U32Type(netmask)
	Flags := BinaryType([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	atl, _ := ATLName2ATL["IpvsServiceAttrList"]
	sattr := CreateAttrListType(atl)
	sattr.Amap["FWMARK"] = &FWMark
	sattr.Amap["FLAGS"] = &Flags
	sattr.Amap["AF"] = &AF
	sattr.Amap["NETMASK"] = &Netmask
	for k, v := range amap {
		sattr.Amap[k] = v
	}
	msg.AttrMap["SERVICE"] = &sattr
	err = ipvs.Sock.Execute(msg)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) AddFWMService(fwmark uint32,
	sched string, af uint16) error {
	paramsMap := make(map[string]SerDes)
	Sched := NulStringType(sched)
	Timeout := U32Type(0)
	paramsMap["SCHED_NAME"] = &Sched
	paramsMap["TIMEOUT"] = &Timeout
	err := ipvs.modifyFWMService("NEW_SERVICE", fwmark,
		af, paramsMap)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) DelFWMService(fwmark uint32, af uint16) error {
	err := ipvs.modifyFWMService("DEL_SERVICE", fwmark, af, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) modifyDest(method string, vip string, vport uint16,
	rip string, rport uint16, protocol uint16, amap map[string]SerDes) error {
	//starts with r - for real's related, v - for vip's
	vaf, vaddr, err := toAFUnion(vip)
	if err != nil {
		return err
	}
	raf, raddr, err := toAFUnion(rip)
	if err != nil {
		return err
	}
	msg, err := ipvs.mt.InitGNLMessageStr(method, ACK_REQUEST)
	if err != nil {
		return err
	}
	vAF := U16Type(vaf)
	vAddr := BinaryType(vaddr)
	rAF := U16Type(raf)
	rAddr := BinaryType(raddr)

	VPort := Net16Type(vport)
	RPort := Net16Type(rport)
	Proto := U16Type(protocol)

	vatl, _ := ATLName2ATL["IpvsServiceAttrList"]
	ratl, _ := ATLName2ATL["IpvsDestAttrList"]
	sattr := CreateAttrListType(vatl)
	rattr := CreateAttrListType(ratl)

	sattr.Amap["AF"] = &vAF
	sattr.Amap["PORT"] = &VPort
	sattr.Amap["PROTOCOL"] = &Proto
	sattr.Amap["ADDR"] = &vAddr

	/*
		XXX(tehnerd): real's port right now is equal to vip's but again it's trivial to fix
		for example in param map you could override amap["PORT"]
	*/
	rattr.Amap["ADDR_FAMILY"] = &rAF
	rattr.Amap["PORT"] = &RPort
	rattr.Amap["ADDR"] = &rAddr

	for k, v := range amap {
		rattr.Amap[k] = v
	}
	msg.AttrMap["SERVICE"] = &sattr
	msg.AttrMap["DEST"] = &rattr
	err = ipvs.Sock.Execute(msg)
	if err != nil {
		return err
	}
	return nil

}

func (ipvs *IpvsClient) AddDest(vip string, port uint16, rip string,
	protocol uint16, weight int32) error {
	return ipvs.AddDestPort(vip, port, rip, port, protocol, weight, IPVS_TUNNELING)
}

func (ipvs *IpvsClient) AddDestPort(vip string, vport uint16, rip string,
	rport uint16, protocol uint16, weight int32, fwd uint32) error {
	paramsMap := make(map[string]SerDes)
	Weight := I32Type(weight)
	//XXX(tehnerd): hardcode, but easy to fix; 2 - tunneling
	FWDMethod := U32Type(fwd)
	LThresh := U32Type(0)
	UThresh := U32Type(0)
	paramsMap["WEIGHT"] = &Weight
	paramsMap["FWD_METHOD"] = &FWDMethod
	paramsMap["L_THRESH"] = &LThresh
	paramsMap["U_THRESH"] = &UThresh
	err := ipvs.modifyDest("NEW_DEST", vip, vport, rip, rport, protocol, paramsMap)
	if err != nil {
		return err
	}
	return err
}

func (ipvs *IpvsClient) UpdateDest(vip string, port uint16, rip string,
	protocol uint16, weight int32) error {
	return ipvs.UpdateDestPort(vip, port, rip, port, protocol, weight, IPVS_TUNNELING)
}

func (ipvs *IpvsClient) UpdateDestPort(vip string, vport uint16, rip string,
	rport uint16, protocol uint16, weight int32, fwd uint32) error {
	paramsMap := make(map[string]SerDes)
	Weight := I32Type(weight)
	//XXX(tehnerd): hardcode, but easy to fix; 2 - tunneling
	FWDMethod := U32Type(fwd)
	LThresh := U32Type(0)
	UThresh := U32Type(0)
	paramsMap["WEIGHT"] = &Weight
	paramsMap["FWD_METHOD"] = &FWDMethod
	paramsMap["L_THRESH"] = &LThresh
	paramsMap["U_THRESH"] = &UThresh
	err := ipvs.modifyDest("SET_DEST", vip, vport, rip, rport, protocol, paramsMap)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) DelDest(vip string, port uint16, rip string,
	protocol uint16) error {
	return ipvs.DelDestPort(vip, port, rip, port, protocol)
}

func (ipvs *IpvsClient) DelDestPort(vip string, vport uint16, rip string,
	rport uint16, protocol uint16) error {
	err := ipvs.modifyDest("DEL_DEST", vip, vport, rip, rport, protocol, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) modifyFWMDest(method string, fwmark uint32,
	rip string, vaf uint16, port uint16, amap map[string]SerDes) error {
	//starts with r - for real's related, v - for vip's
	raf, raddr, err := toAFUnion(rip)
	if err != nil {
		return err
	}
	msg, err := ipvs.mt.InitGNLMessageStr(method, ACK_REQUEST)
	if err != nil {
		return err
	}
	vAF := U16Type(vaf)

	rAF := U16Type(raf)
	rAddr := BinaryType(raddr)
	Port := Net16Type(port)

	FWMark := U32Type(fwmark)

	vatl, _ := ATLName2ATL["IpvsServiceAttrList"]
	ratl, _ := ATLName2ATL["IpvsDestAttrList"]

	sattr := CreateAttrListType(vatl)
	rattr := CreateAttrListType(ratl)

	sattr.Amap["FWMARK"] = &FWMark
	sattr.Amap["AF"] = &vAF

	rattr.Amap["ADDR_FAMILY"] = &rAF
	rattr.Amap["ADDR"] = &rAddr
	rattr.Amap["PORT"] = &Port

	for k, v := range amap {
		rattr.Amap[k] = v
	}
	msg.AttrMap["SERVICE"] = &sattr
	msg.AttrMap["DEST"] = &rattr
	err = ipvs.Sock.Execute(msg)
	if err != nil {
		return err
	}
	return nil
}

/*
func (ipvs *IpvsClient) modifyFWMDest(method string, fwmark uint32,
	rip string, vaf uint16, port uint16, amap map[string]SerDes) {

*/

func (ipvs *IpvsClient) AddFWMDest(fwmark uint32, rip string, vaf uint16,
	port uint16, weight int32) error {
	return ipvs.AddFWMDestFWD(fwmark, rip, vaf, port, weight, IPVS_TUNNELING)
}

func (ipvs *IpvsClient) AddFWMDestFWD(fwmark uint32, rip string, vaf uint16,
	port uint16, weight int32, fwd uint32) error {
	paramsMap := make(map[string]SerDes)
	Weight := I32Type(weight)
	//XXX(tehnerd): hardcode, but easy to fix; 2 - tunneling
	FWDMethod := U32Type(fwd)
	LThresh := U32Type(0)
	UThresh := U32Type(0)
	paramsMap["WEIGHT"] = &Weight
	paramsMap["FWD_METHOD"] = &FWDMethod
	paramsMap["L_THRESH"] = &LThresh
	paramsMap["U_THRESH"] = &UThresh
	err := ipvs.modifyFWMDest("NEW_DEST", fwmark, rip, vaf, port, paramsMap)
	if err != nil {
		return err
	}
	return nil
}
func (ipvs *IpvsClient) UpdateFWMDest(fwmark uint32, rip string, vaf uint16,
	port uint16, weight int32) error {
	return ipvs.UpdateFWMDestFWD(fwmark, rip, vaf, port, weight, IPVS_TUNNELING)
}

func (ipvs *IpvsClient) UpdateFWMDestFWD(fwmark uint32, rip string, vaf uint16,
	port uint16, weight int32, fwd uint32) error {
	paramsMap := make(map[string]SerDes)
	Weight := I32Type(weight)
	//XXX(tehnerd): hardcode, but easy to fix; 2 - tunneling
	FWDMethod := U32Type(fwd)
	LThresh := U32Type(0)
	UThresh := U32Type(0)
	paramsMap["WEIGHT"] = &Weight
	paramsMap["FWD_METHOD"] = &FWDMethod
	paramsMap["L_THRESH"] = &LThresh
	paramsMap["U_THRESH"] = &UThresh
	err := ipvs.modifyFWMDest("SET_DEST", fwmark, rip, vaf, port, paramsMap)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) DelFWMDest(fwmark uint32, rip string, vaf uint16,
	port uint16) error {
	err := ipvs.modifyFWMDest("DEL_DEST", fwmark, rip, vaf, port, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ipvs *IpvsClient) Exit() {
	ipvs.Sock.Close()
}
