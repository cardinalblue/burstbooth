package main

import (
	"flag"

	"github.com/golang/glog"

	"github.com/cardinalblue/burstbooth"
)

func main() {
	flag.Parse()
	err := burstbooth.CreateDDBTables()
	if err != nil {
		glog.Fatalf(err.Error())
	}
}
