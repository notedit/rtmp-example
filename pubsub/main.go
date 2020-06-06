package main

import (
	"fmt"
	"github.com/notedit/rtmp/format/rtmp"
	"github.com/notedit/rtmp/pubsub"
	"net"
	"sync"
	"time"
)

var lock sync.RWMutex
var streams = map[string]*pubsub.PubSub{}



func startRtmp() {

	lis, err := net.Listen("tcp", ":1935")
	if err != nil {
		panic(err)
	}

	s := rtmp.NewServer()

	s.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
		es := rtmp.EventString[e]
		fmt.Println(nc.LocalAddr(), nc.RemoteAddr(), es)
	}

	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {

		fmt.Println(c.URL.Path)

		if c.Publishing {
			pubsuber := &pubsub.PubSub{}
			streams[c.URL.Path] = pubsuber
			pubsuber.SetPub(c)
			delete(streams,c.URL.Path)
		} else {
			pubsuber := streams[c.URL.Path]
			if pubsuber != nil {
				pubsuber.AddSub(c.CloseNotify(),c)
			} else {
				nc.Close()
			}
		}
	}

	for {
		nc, err := lis.Accept()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		go s.HandleNetConn(nc)
	}
}

func main() {
	
	startRtmp()
}
