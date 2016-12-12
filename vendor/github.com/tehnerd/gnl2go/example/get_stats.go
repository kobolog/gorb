package main

import (
	"fmt"
	"gnl2go/gnl2go"
)

/* we can ran this as binary for unit tests (for example build it on
   on one machine(mac or win) and run on other(linux), probably
   later will convert into ipvs_test.go */
func main() {
	fmt.Println("Output")
	ipvs := new(gnl2go.IpvsClient)
	ipvs.Init()
	defer ipvs.Exit()
	data, _ := ipvs.GetAllStatsBrief()
	for k, v := range data {
		fmt.Println(k)
		stats := v.GetStats()
		for nk, nv := range stats {
			fmt.Printf("%v : %v   ", nk, nv)
		}
		fmt.Printf("\n")
	}
	fmt.Println("done")
}
