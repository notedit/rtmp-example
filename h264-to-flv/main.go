package main

import (
	"fmt"
	"github.com/notedit/gst"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/codec/h264"
	"github.com/notedit/rtmp/format/flv"
	"os"
	"time"
)


const gstVideoPipeline = "videotestsrc ! video/x-raw,framerate=15/1 ! x264enc aud=false bframes=0 speed-preset=veryfast key-int-max=30 ! video/x-h264,stream-format=avc,profile=baseline ! h264parse ! appsink name=videosink "

type Sample struct {
	Data     []byte
	Video    bool
	Duration uint64
}


func pullVideoSample(element *gst.Element, out chan *Sample) {

	for {

		sle, err := element.PullSample()
		if err != nil {
			if element.IsEOS() == true {
				fmt.Println("eos")
				return
			} else {
				fmt.Println(err)
				continue
			}
		}

		sample := &Sample{
			Data:     sle.Data,
			Video:    true,
			Duration: sle.Duration,
		}

		out <- sample
	}
}


func main() {

	samples := make(chan *Sample, 10)

	err := gst.CheckPlugins([]string{"x264", "videoparsersbad"})

	if err != nil {
		panic(err)
	}

	vpipeline, err := gst.ParseLaunch(gstVideoPipeline)

	if err != nil {
		panic(err)
	}

	velement := vpipeline.GetByName("videosink")
	vpipeline.SetState(gst.StatePlaying)

	go pullVideoSample(velement, samples)


	file, err := os.Create("record.flv")
	if err != nil {
		panic(err)
	}

	muxer := flv.NewMuxer(file)
	muxer.HasAudio = false
	muxer.HasVideo = true
	muxer.Publishing = true
	muxer.WriteFileHeader()



	h264Codec := h264.NewCodec()
	decodeConfig := av.Packet{
		Type:       av.H264DecoderConfig,
		IsKeyFrame: true,
	}


	start := time.Now()

	for sample := range samples {

		if sample.Video {
			nalus, _ := h264.SplitNALUs(sample.Data)
			nalus_ := make([][]byte, 0)
			keyframe := false
			for _, nalu := range nalus {
				typ := h264.NALUType(nalu)
				fmt.Println(h264.NALUTypeString(typ))
				switch typ {
				case h264.NALU_SPS:
					h264Codec.AddSPSPPS(nalu)
				case h264.NALU_PPS:
					h264Codec.AddSPSPPS(nalu)
					decodeConfig.Data = make([]byte, 5000)
					var len int
					h264Codec.ToConfig(decodeConfig.Data, &len)
					decodeConfig.Data = decodeConfig.Data[0:len]
					muxer.WritePacket(decodeConfig)
				case h264.NALU_IDR:
					nalus_ = append(nalus_, nalu)
					keyframe = true
				case h264.NALU_NONIDR:
					nalus_ = append(nalus_, nalu)
					keyframe = false
					break
				}
			}

			duration := time.Since(start)
			data := h264.FillNALUsAVCC(nalus_)
			pkt := av.Packet{
				Type:       av.H264,
				IsKeyFrame: keyframe,
				Time:       duration,
				CTime:      duration,
				Data:       data,
			}
			muxer.WritePacket(pkt)
		}
	}

	vpipeline.SetState(gst.StateNull)

	vpipeline = nil
	velement = nil

}
