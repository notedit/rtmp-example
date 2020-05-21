package main

import (
	"fmt"
	"github.com/notedit/gst"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/codec/h264"
	"github.com/notedit/rtmp/format/rtmp"
	"net"
	"time"
)

type stream struct {
	videoSeq av.Packet
	audioSeq av.Packet
	conn     *rtmp.Conn
}

var pub *stream

const gstPipeline = "videotestsrc ! video/x-raw,framerate=15/1 ! x264enc aud=false bframes=0 speed-preset=veryfast key-int-max=15 ! video/x-h264,stream-format=avc,profile=baseline ! h264parse ! appsink name=videosink "

func startPush() {

	err := gst.CheckPlugins([]string{"x264", "videoparsersbad"})

	if err != nil {
		panic(err)
	}

	pipeline, err := gst.ParseLaunch(gstPipeline)

	if err != nil {
		panic(err)
	}

	element := pipeline.GetByName("videosink")
	pipeline.SetState(gst.StatePlaying)

	rtmpClient := rtmp.NewClient()

	var c *rtmp.Conn
	c, _, err = rtmpClient.Dial("rtmp://localhost/live/live", rtmp.PrepareWriting)

	if err != nil {
		panic(err)
	}

	h264Codec := h264.NewCodec()
	decodeConfig := av.Packet{
		Type:       av.H264DecoderConfig,
		IsKeyFrame: true,
	}

	start := time.Now()

	for {
		sample, err := element.PullSample()
		if err != nil {
			if element.IsEOS() == true {
				fmt.Println("eos")
				return
			} else {
				fmt.Println(err)
				continue
			}
		}

		nalus, _ := h264.SplitNALUs(sample.Data)
		nalus_ := make([][]byte, 0)
		keyframe := false
		for _, nalu := range nalus {
			typ := h264.NALUType(nalu)
			switch typ {
			case h264.NALU_SPS:
				h264Codec.AddSPSPPS(nalu)
			case h264.NALU_PPS:
				h264Codec.AddSPSPPS(nalu)
				decodeConfig.Data = make([]byte, 2048)
				var len int
				h264Codec.ToConfig(decodeConfig.Data, &len)
				decodeConfig.Data = decodeConfig.Data[0:len]
				c.WritePacket(decodeConfig)
			case h264.NALU_IDR:
				nalus_ = append(nalus_, nalu)
			case h264.NALU_NONIDR:
				nalus_ = append(nalus_, nalu)
				keyframe = true
				break
			}
			//fmt.Println("client ",h264.NALUTypeString(h264.NALUType(nalu)))
		}

		duration := time.Since(start)
		data := h264.FillNALUsAVCC(nalus_)
		pkt := av.Packet{
			Type: av.H264,
			IsKeyFrame:keyframe,
			Time: duration,
			CTime: duration,
			Data: data,
		}

		c.WritePacket(pkt)
		//fmt.Println("client ",pkt)
	}

	pipeline.SetState(gst.StateNull)

	pipeline = nil
	element = nil
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

		if !c.Publishing {
			return
		}

		pub := &stream{}
		pub.conn = c

		for {
			pkt, err := c.ReadPacket()
			if err != nil {
				return
			}
			switch pkt.Type {
			case av.H264DecoderConfig:
				pub.videoSeq = pkt
				fmt.Println("server  sps  pps ========")
			case av.H264:
				fmt.Println("server ", pkt.Time)
			case av.AACDecoderConfig:
				pub.audioSeq = pkt
			case av.AAC:
				fmt.Println("server ", pkt.Time)
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

	go startRtmp()

	// sleep for a while
	time.Sleep(time.Second)

	startPush()
}
