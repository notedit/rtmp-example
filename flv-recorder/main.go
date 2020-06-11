package main

import (
	"fmt"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/format/flv"
	"github.com/notedit/rtmp/format/rtmp"
	"net"
	"os"
	"time"
)

type stream struct {
	videoSeq av.Packet
	audioSeq av.Packet
	conn     *rtmp.Conn
}

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

			file, err := os.Create("record.flv")
			if err != nil {
				panic(err)
			}

			muxer := flv.NewMuxer(file)
			muxer.HasAudio = true
			muxer.HasVideo = true
			muxer.Publishing = true
			muxer.WriteFileHeader()

			for {
				pkt, err := c.ReadPacket()
				if err != nil {
					return
				}
				switch pkt.Type {
				case av.H264DecoderConfig:
					muxer.WritePacket(pkt)
					fmt.Println("server  sps  pps ========")
				case av.H264:
					muxer.WritePacket(pkt)
					fmt.Println("server h264 ", pkt.Time)
				case av.AACDecoderConfig:
					muxer.WritePacket(pkt)
				case av.AAC:
					fmt.Println("server aac", pkt.Time)
					muxer.WritePacket(pkt)
				}
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
