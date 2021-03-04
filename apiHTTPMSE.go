package main

import (
	"context"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

var timeout_novideo time.Duration = 20
var tiemout_ws time.Duration = 10

//HTTPAPIServerStreamMSE func
func HTTPAPIServerStreamMSE(ws *websocket.Conn) {
	streamID := ws.Request().FormValue("uuid")
	channelID := ws.Request().FormValue("channel")

	// close websocket.
	defer func() {
		_ = ws.Close()
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Close",
		}).Debugln("Client Full Exit")
	}()

	// check stream status
	// if !Storage.StreamChannelExist(streamID, channelID)
	ch, err := Storage.StreamChannelInfo(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	log.WithFields(logrus.Fields{
		"module":  "http_mse",
		"stream":  streamID,
		"channel": channelID,
		"func":    "HTTPAPIServerStreamMSE",
		"call":    "StreamChannelRun",
	}).Debugln("play stream ---> ", streamID, channelID)
	Storage.StreamChannelRun(streamID, channelID)

	// get stream av.Codec
	codecs, err := Storage.StreamChannelCodecs(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}

	// add client/player
	cid, avChanR, _, err := Storage.ClientAdd(streamID, channelID, MSE)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "ClientAdd",
		}).Errorln(err.Error())
		return
	}
	defer Storage.ClientDelete(streamID, cid, channelID)

	// set websocket timeout for write av.Pkt
	err = ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "SetWriteDeadline",
		}).Errorln(err.Error())
		return
	}
	// init MSE muxer
	muxerMSE := mp4f.NewMuxer(nil)
	err = muxerMSE.WriteHeader(codecs)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
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
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send",
		}).Errorln(err.Error())
		return
	}
	err = websocket.Message.Send(ws, init)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send MSE meta",
		}).Errorln(err.Error())
		return
	}

	// make writeable chan for read av.pkt
	avChanW := make(chan *av.Packet, 10)

	// creat recv av.pkt goroutine
	ctx, cancel := context.WithCancel(context.Background())
	go StreamServerStreamChannel(ctx, streamID, channelID, avChanR, avChanW, 20)
	// cancel av.packet send goroutine.
	defer func() {
		cancel()
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "recv avpkt",
		}).Infoln("Cancelled av send goroutine.")
	}()

	for {
		select {
		case avPkt := <-avChanW:
			// log.Println("got avPkt. ", avPkt.IsKeyFrame, len(avPkt.Data))

			ready, buf, err := muxerMSE.WritePacket(*avPkt, true)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "http_mse",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamMSE",
					"call":    "WritePacket",
				}).Errorln(err.Error())
				return
			}
			if ready {
				if ch.Debug && avPkt.IsKeyFrame {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Send frame, key:%v, len:%v, DTS:%v, Dur:%v", avPkt.IsKeyFrame, len(buf), avPkt.Time, avPkt.Duration)
				}

				err := ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "SetWriteDeadline",
					}).Errorln(err.Error())
					return
				}
				err = websocket.Message.Send(ws, buf)
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "Send MSE AV",
					}).Errorln(err.Error())
					return
				}
			} else {
				if ch.Debug && len(buf) > 0 {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Drop frame, key:%v, len:%v, DTS:%v, Dur:%v", avPkt.IsKeyFrame, len(buf), avPkt.Time, avPkt.Duration)
				}
			}
			// default:
			// 	var message string
			// 	err := websocket.Message.Receive(ws, &message)
			// 	if err != nil {
			// 		log.WithFields(logrus.Fields{
			// 			"module":  "http_mse",
			// 			"stream":  streamID,
			// 			"channel": channelID,
			// 			"func":    "HTTPAPIServerStreamMSE",
			// 			"call":    "Receive",
			// 		}).Errorln(err.Error())
			// 		return
			// 	}

			// 	log.WithFields(logrus.Fields{
			// 	"module":  "http_mse",
			// 	"stream":  streamID,
			// 	"channel": channelID,
			// 	"func":    "HTTPAPIServerStreamMSE",
			// 	"call":    "recv avpkt",
			// }).Debugln("WS recv msg: ", message)
		}
	}

	// var videoStart bool
	// controlExit := make(chan bool, 10)
	// go func() {
	// 	defer func() {
	// 		controlExit <- true
	// 	}()
	// 	for {
	// 		var message string
	// 		err := websocket.Message.Receive(ws, &message)
	// 		if err != nil {
	// 			log.WithFields(logrus.Fields{
	// 				"module":  "http_mse",
	// 				"stream":  streamID,
	// 				"channel": channelID,
	// 				"func":    "HTTPAPIServerStreamMSE",
	// 				"call":    "Receive",
	// 			}).Errorln(err.Error())
	// 			return
	// 		}

	// 		log.WithFields(logrus.Fields{
	// 			"module":  "http_mse",
	// 			"stream":  streamID,
	// 			"channel": channelID,
	// 			"func":    "HTTPAPIServerStreamMSE",
	// 			"call":    "recv avpkt",
	// 		}).Debugln("WS recv msg: ", message)
	// 	}
	// }()

	// noVideo := time.NewTimer(timeout_novideo * time.Second)
	// for {
	// 	select {
	// 	case <-controlExit:
	// 		log.WithFields(logrus.Fields{
	// 			"module":  "http_mse",
	// 			"stream":  streamID,
	// 			"channel": channelID,
	// 			"func":    "HTTPAPIServerStreamMSE",
	// 			"call":    "controlExit",
	// 		}).Errorln("Client Reader Exit")
	// 		return
	// 	case <-noVideo.C:
	// 		log.WithFields(logrus.Fields{
	// 			"module":  "http_mse",
	// 			"stream":  streamID,
	// 			"channel": channelID,
	// 			"func":    "HTTPAPIServerStreamMSE",
	// 			"call":    "ErrorStreamNoVideo",
	// 		}).Errorln(ErrorStreamNoVideo.Error(), videoStart)
	// 		return
	// 	case avpkt := <-ch:
	// 		if avpkt.IsKeyFrame {
	// 			videoStart = true
	// 		}
	// 		noVideo.Reset(timeout_novideo * time.Second)

	// 		if !videoStart {
	// 			continue
	// 		}

	// 		ready, buf, err := muxerMSE.WritePacket(*avpkt, false)
	// 		if err != nil {
	// 			log.WithFields(logrus.Fields{
	// 				"module":  "http_mse",
	// 				"stream":  streamID,
	// 				"channel": channelID,
	// 				"func":    "HTTPAPIServerStreamMSE",
	// 				"call":    "WritePacket",
	// 			}).Errorln(err.Error())
	// 			return
	// 		}
	// 		if ready {
	// 			if avpkt.IsKeyFrame {
	// 				log.WithFields(logrus.Fields{
	// 					"module":  "http_mse",
	// 					"stream":  streamID,
	// 					"channel": channelID,
	// 					"func":    "HTTPAPIServerStreamMSE",
	// 					"call":    "recv avpkt",
	// 				}).Debugf("Send frame, key:%v, len:%v, DTS:%v, Dur:%v", avpkt.IsKeyFrame, len(buf), avpkt.Time, avpkt.Duration)
	// 			}

	// 			err := ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
	// 			if err != nil {
	// 				log.WithFields(logrus.Fields{
	// 					"module":  "http_mse",
	// 					"stream":  streamID,
	// 					"channel": channelID,
	// 					"func":    "HTTPAPIServerStreamMSE",
	// 					"call":    "SetWriteDeadline",
	// 				}).Errorln(err.Error())
	// 				return
	// 			}
	// 			err = websocket.Message.Send(ws, buf)
	// 			if err != nil {
	// 				log.WithFields(logrus.Fields{
	// 					"module":  "http_mse",
	// 					"stream":  streamID,
	// 					"channel": channelID,
	// 					"func":    "HTTPAPIServerStreamMSE",
	// 					"call":    "Send",
	// 				}).Errorln(err.Error())
	// 				return
	// 			}
	// 		} else {
	// 			if len(buf) > 0 {
	// 				log.WithFields(logrus.Fields{
	// 					"module":  "http_mse",
	// 					"stream":  streamID,
	// 					"channel": channelID,
	// 					"func":    "HTTPAPIServerStreamMSE",
	// 					"call":    "recv avpkt",
	// 				}).Debugf("Drop frame, key:%v, len:%v, DTS:%v, Dur:%v", avpkt.IsKeyFrame, len(buf), avpkt.Time, avpkt.Duration)
	// 			}
	// 		}

	// 	}
	// }
}
