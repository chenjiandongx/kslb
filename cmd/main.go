package main

import "github.com/chenjiandongx/kslb"

func main() {
	slb := kslb.NewSLB(nil)
	slb.Run()
}
