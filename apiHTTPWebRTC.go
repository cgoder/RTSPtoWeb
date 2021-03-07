package main

import (
	"context"
	"time"

	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//HTTPAPIServerStreamWebRTC stream video over WebRTC
func HTTPAPIServerStreamWebRTC(c *gin.Context) {
	streamID := ws.Request().FormValue("uuid")
	channelID := ws.Request().FormValue("channel")

	if !Storage.StreamChannelExist(streamID, channelID) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
	Storage.StreamChannelRun(context.Background(), streamID, channelID)
	codecs, err := Storage.StreamChannelCodecs(streamID, channelID)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{})
	answer, err := muxerWebRTC.WriteHeader(codecs, c.PostForm("data"))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "Write",
		}).Errorln(err.Error())
		return
	}
	go func() {
		cid, ch, _, err := Storage.ClientAdd(streamID, channelID, WEBRTC)
		if err != nil {
			c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_webrtc",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamWebRTC",
				"call":    "ClientAdd",
			}).Errorln(err.Error())
			return
		}
		defer Storage.ClientDelete(streamID, cid, channelID)
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNoVideo.Error()})
				log.WithFields(logrus.Fields{
					"module":  "http_webrtc",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamWebRTC",
					"call":    "ErrorStreamNoVideo",
				}).Errorln(ErrorStreamNoVideo.Error())
				return
			case pck := <-ch:
				if pck.IsKeyFrame {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart {
					continue
				}
				err = muxerWebRTC.WritePacket(*pck)
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_webrtc",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamWebRTC",
						"call":    "WritePacket",
					}).Errorln(err.Error())
					return
				}
			}
		}
	}()
}
