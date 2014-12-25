package main

import (
	"flag"

	"github.com/golang/glog"

	"hack20141225"
)

func main() {
	flag.Parse()
	err := hack.CreateDDBTables()
	if err != nil {
		glog.Fatalf(err.Error())
	}
}
