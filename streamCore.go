package main

import (
	"errors"
	"strings"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format"
	"github.com/deepch/vdk/format/rtmp"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/sirupsen/logrus"
)

//StreamServerRunStreamDo stream run do mux
func StreamServerRunStreamDo(streamID string, channelID string) {
	var status int
	defer func() {
		//TODO fix it no need unlock run if delete stream
		if status != 2 {
			Storage.StreamChannelUnlock(streamID, channelID)
		}
	}()
	for {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamDo",
			"call":    "Run",
		}).Infoln("Run stream ", streamID, channelID)
		opt, err := Storage.StreamChannelControl(streamID, channelID)
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamChannelControl",
				"call":    "Exit",
			}).Infoln("Exit", err)
			return
		}
		if opt.OnDemand && !Storage.ClientHas(streamID, channelID) {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamDo",
				"call":    "ClientHas",
			}).Infoln("Stop stream no client")
			return
		}
		// status, err = StreamServerRunStream(streamID, channelID, opt)
		status, err = StreamServerRunStream(streamID, channelID, opt)
		if status > 0 {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamDo",
				"call":    "StreamServerRunStream",
			}).Infoln("Stream exit by signal or not client")
			return
		}
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamDo",
				"call":    "Restart",
			}).Errorln("Stream error restart stream", err)
		}
		time.Sleep(2 * time.Second)

	}
}

func init() {
	format.RegisterAll()
}

func StreamServerRunStream(streamID string, channelID string, opt *ChannelST) (int, error) {
	if strings.HasPrefix(opt.URL, "rtmp://") {
		return StreamServerRunStreamRtmp(streamID, channelID, opt)
	} else if strings.HasPrefix(opt.URL, "rtsp://") {
		return StreamServerRunStreamRtsp(streamID, channelID, opt)
	} else {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStream",
			"call":    "protocal select",
		}).Errorln("Unsupport protocol. ")
		return 0, errors.New("Unsupport protocol")
	}
}

//StreamServerRunStream core stream
func StreamServerRunStreamRtsp(streamID string, channelID string, opt *ChannelST) (int, error) {
	keyTest := time.NewTimer(20 * time.Second)
	checkClients := time.NewTimer(20 * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: opt.URL, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 5 * time.Second, Debug: opt.Debug, OutgoingProxy: true})
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamRtsp",
			"call":    "Start",
		}).Errorln("RTSPClient.Dial fail. ", err)
		return 0, err
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtsp",
		"call":    "Start",
	}).Debugln("RTSPClient.SDPRaw---> ", JsonFormat(RTSPClient.SDPRaw))
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtsp",
		"call":    "Start",
	}).Debugln("RTSPClient.CodecData---> ", JsonFormat(RTSPClient.CodecData))

	Storage.StreamChannelStatus(streamID, channelID, ONLINE)
	defer func() {
		RTSPClient.Close()
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	if len(RTSPClient.CodecData) > 0 {
		Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
		opt.updated <- true
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtsp",
		"call":    "Start",
	}).Debugln("Success connection RTSP")

	for {
		select {
		//Check stream have clients
		case <-checkClients.C:
			if opt.OnDemand && !Storage.ClientHas(streamID, channelID) {

				return 1, ErrorStreamNoClients
			}
			checkClients.Reset(20 * time.Second)
		//Check stream send key
		case <-keyTest.C:
			return 0, ErrorStreamNoVideo
		//Read core signals
		case signals := <-opt.signals:
			switch signals {
			case SignalStreamStop:
				return 2, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				return 0, ErrorStreamRestart
			case SignalStreamClient:

				return 1, ErrorStreamNoClients
			}
		//Read rtsp signals
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
			case rtspv2.SignalStreamRTPStop:
				return 0, ErrorStreamStopRTSPSignal
			}
		case packetRTP := <-RTSPClient.OutgoingProxyQueue:
			keyTest.Reset(20 * time.Second)
			Storage.StreamChannelCastProxy(streamID, channelID, packetRTP)
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, packetAV.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = packetAV.Time
			}
			Seq = append(Seq, packetAV)
			Storage.StreamChannelCast(streamID, channelID, packetAV)
		}
	}
}

func StreamServerRunStreamRtmp(streamID string, channelID string, opt *ChannelST) (int, error) {
	keyTest := time.NewTimer(20 * time.Second)
	checkClients := time.NewTimer(20 * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	RTMPConn, err := rtmp.Dial(opt.URL)
	// RTMPConn, err := rtmp.DialTimeout(opt.URL, 20)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamRtmp",
			"call":    "rtmp.Dial",
		}).Errorln("RTMP Dial ---> ", JsonFormat(RTMPConn.URL), err)
		return 0, err
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtmp",
		"call":    "Start",
	}).Debugln("RTMP Conn---> ", JsonFormat(RTMPConn.URL))

	// if err := RTMPConn.Prepare(); err != nil {
	// 	return 0, err
	// }

	t1 := time.Now().Local().UTC()
	streams, err := RTMPConn.Streams()
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamRtmp",
			"call":    "RTMPConn.Streams",
		}).Errorln("RTMP get stream codec err. ", err)
		return 0, err
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtmp",
		"call":    "RTMPConn.Streams",
	}).Debugln("rtmp get stream codec DONE! time: ", time.Now().Local().UTC().Sub(t1).String())

	if len(streams) > 0 {
		Storage.StreamChannelCodecsUpdate(streamID, channelID, streams, nil)
		opt.updated <- true
	}

	// log.WithFields(logrus.Fields{
	// 	"module":  "core",
	// 	"stream":  streamID,
	// 	"channel": channelID,
	// 	"func":    "StreamServerRunStreamRtmp",
	// 	"call":    "Start",
	// }).Debugln("Success connection RTMP")

	Storage.StreamChannelStatus(streamID, channelID, ONLINE)
	defer func() {
		RTMPConn.Close()
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	for {
		select {
		//Check stream have clients
		case <-checkClients.C:
			if opt.OnDemand && !Storage.ClientHas(streamID, channelID) {
				return 1, ErrorStreamNoClients
			}
			checkClients.Reset(20 * time.Second)
		//Check stream send key
		case <-keyTest.C:
			return 0, ErrorStreamNoVideo
		//Read core signals
		case signals := <-opt.signals:
			switch signals {
			case SignalStreamStop:
				return 2, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				return 0, ErrorStreamRestart
			case SignalStreamClient:
				return 1, ErrorStreamNoClients
			}
		default:
			pktRTMP, err := RTMPConn.ReadPacket()
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamServerRunStreamRtmp",
					"call":    "ReadPacket",
				}).Errorln("ReadPacket error ", err)
			}

			keyTest.Reset(20 * time.Second)
			Storage.StreamChannelCastProxy(streamID, channelID, &pktRTMP.Data)

			if pktRTMP.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, pktRTMP.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = pktRTMP.Time
			}
			Seq = append(Seq, &pktRTMP)
			Storage.StreamChannelCast(streamID, channelID, &pktRTMP)

		}
	}
}
