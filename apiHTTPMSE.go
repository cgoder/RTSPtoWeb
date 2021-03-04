package main

import (
	"time"

	"github.com/deepch/vdk/format/mp4f"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

var timeout_novideo time.Duration = 20
var tiemout_ws time.Duration = 10

//HTTPAPIServerStreamMSE func
func HTTPAPIServerStreamMSE(ws *websocket.Conn) {
	defer func() {
		_ = ws.Close()
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Close",
		}).Debugln("Client Full Exit")
	}()
	if !Storage.StreamChannelExist(ws.Request().FormValue("uuid"), ws.Request().FormValue("channel")) {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	log.WithFields(logrus.Fields{
		"module":  "http_mse",
		"stream":  ws.Request().FormValue("uuid"),
		"channel": ws.Request().FormValue("channel"),
		"func":    "HTTPAPIServerStreamMSE",
		"call":    "StreamChannelRun",
	}).Debugln("play stream ---> ", ws.Request().FormValue("uuid"), ws.Request().FormValue("channel"))

	Storage.StreamChannelRun(ws.Request().FormValue("uuid"), ws.Request().FormValue("channel"))

	codecs, err := Storage.StreamChannelCodecs(ws.Request().FormValue("uuid"), ws.Request().FormValue("channel"))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}

	cid, ch, _, err := Storage.ClientAdd(ws.Request().FormValue("uuid"), ws.Request().FormValue("channel"), MSE)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "ClientAdd",
		}).Errorln(err.Error())
		return
	}
	defer Storage.ClientDelete(ws.Request().FormValue("uuid"), cid, ws.Request().FormValue("channel"))

	err = ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "SetWriteDeadline",
		}).Errorln(err.Error())
		return
	}

	muxerMSE := mp4f.NewMuxer(nil)
	err = muxerMSE.WriteHeader(codecs)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	meta, init := muxerMSE.GetInit(codecs)
	err = websocket.Message.Send(ws, append([]byte{9}, meta...))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send",
		}).Errorln(err.Error())
		return
	}
	err = websocket.Message.Send(ws, init)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  ws.Request().FormValue("uuid"),
			"channel": ws.Request().FormValue("channel"),
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send",
		}).Errorln(err.Error())
		return
	}
	var videoStart bool
	controlExit := make(chan bool, 10)
	go func() {
		defer func() {
			controlExit <- true
		}()
		for {
			var message string
			err := websocket.Message.Receive(ws, &message)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "http_mse",
					"stream":  ws.Request().FormValue("uuid"),
					"channel": ws.Request().FormValue("channel"),
					"func":    "HTTPAPIServerStreamMSE",
					"call":    "Receive",
				}).Errorln(err.Error())
				return
			}

			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  ws.Request().FormValue("uuid"),
				"channel": ws.Request().FormValue("channel"),
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "recv avpkt",
			}).Debugln("WS recv msg: ", message)
		}
	}()
	noVideo := time.NewTimer(timeout_novideo * time.Second)
	for {
		select {
		case <-controlExit:
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  ws.Request().FormValue("uuid"),
				"channel": ws.Request().FormValue("channel"),
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "controlExit",
			}).Errorln("Client Reader Exit")
			return
		case <-noVideo.C:
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  ws.Request().FormValue("uuid"),
				"channel": ws.Request().FormValue("channel"),
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "ErrorStreamNoVideo",
			}).Errorln(ErrorStreamNoVideo.Error(), videoStart)
			return
		case pck := <-ch:
			if pck.IsKeyFrame {
				videoStart = true
			}
			noVideo.Reset(timeout_novideo * time.Second)

			if !videoStart {
				continue
			}

			ready, buf, err := muxerMSE.WritePacket(*pck, false)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "http_mse",
					"stream":  ws.Request().FormValue("uuid"),
					"channel": ws.Request().FormValue("channel"),
					"func":    "HTTPAPIServerStreamMSE",
					"call":    "WritePacket",
				}).Errorln(err.Error())
				return
			}
			if ready {
				if pck.IsKeyFrame {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  ws.Request().FormValue("uuid"),
						"channel": ws.Request().FormValue("channel"),
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Send frame, key:%v, len:%v, DTS:%v, Dur:%v", pck.IsKeyFrame, len(buf), pck.Time, pck.Duration)
				}

				err := ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  ws.Request().FormValue("uuid"),
						"channel": ws.Request().FormValue("channel"),
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "SetWriteDeadline",
					}).Errorln(err.Error())
					return
				}
				err = websocket.Message.Send(ws, buf)
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  ws.Request().FormValue("uuid"),
						"channel": ws.Request().FormValue("channel"),
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "Send",
					}).Errorln(err.Error())
					return
				}
			} else {
				if len(buf) > 0 {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  ws.Request().FormValue("uuid"),
						"channel": ws.Request().FormValue("channel"),
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Drop frame, key:%v, len:%v, DTS:%v, Dur:%v", pck.IsKeyFrame, len(buf), pck.Time, pck.Duration)
				}
			}

		}
	}
}
