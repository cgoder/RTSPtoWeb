package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cgoder/vdk/av"
	"github.com/cgoder/vdk/format"
	"github.com/cgoder/vdk/format/rtmp"
	"github.com/cgoder/vdk/format/rtspv2"
	"github.com/sirupsen/logrus"
)

var (
	timeoutClientCheck int = 60
	timeoutAvReadCheck int = 20
)

func init() {
	format.RegisterAll()
}

//StreamServerRunStreamDo stream run do mux
// streaming goroutine
func StreamServerRunStreamDo(ctx context.Context, streamID string, channelID string) {
	fmt.Println("StreamServerRunStreamDo--->>>>")

	// get stream channel
	channel, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelGet",
			"call":    "Exit",
		}).Infoln("Exit", err)
		return
	}

	// judge stream clients
	if channel.OnDemand && !Storage.ClientHas(streamID, channelID) {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamDo",
			"call":    "ClientHas",
		}).Infoln("Stream end has no client")
		return
	}

	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamDo",
		"call":    "Run",
	}).Infoln("Run stream-> ", streamID, channelID)

	go StreamServerRunStream(ctx, streamID, channelID, channel)

}

//StreamServerRunStream streaming by protocal
func StreamServerRunStream(ctx context.Context, streamID string, channelID string, channel *ChannelST) error {
	if strings.HasPrefix(channel.URL, "rtmp://") {
		_, err := StreamServerRunStreamRtmp(ctx, streamID, channelID, channel)
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStream",
				"call":    "StreamServerRunStreamRtmp",
			}).Errorln(err)
		}
		return err
	} else if strings.HasPrefix(channel.URL, "rtsp://") {
		_, err := StreamServerRunStreamRtsp(ctx, streamID, channelID, channel)
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStream",
				"call":    "StreamServerRunStreamRtsp",
			}).Errorln(err)
		}
		return err
	} else {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStream",
			"call":    "protocal select",
		}).Errorln("Unsupport protocol. ", channel.URL)
		return errors.New("Unsupport protocol")
	}
}

//StreamServerRunStreamRtsp rtsp stream goroutine
func StreamServerRunStreamRtsp(ctx context.Context, streamID string, channelID string, channel *ChannelST) (int, error) {
	// rtsp client dial
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: channel.URL, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 5 * time.Second, Debug: false, OutgoingProxy: false})
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

	// rtsp codec update
	if len(RTSPClient.CodecData) > 0 {
		Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
		channel.signals <- SignalStreamCodecUpdate
		// log.WithFields(logrus.Fields{
		// 	"module":  "core",
		// 	"stream":  streamID,
		// 	"channel": channelID,
		// 	"func":    "StreamServerRunStreamRtsp",
		// 	"call":    "StreamChannelCodecsUpdate",
		// }).Debugln("RTSPClient.CodecData update send ---> ", JsonFormat(RTSPClient.CodecData))
	}

	// stream status update
	Storage.StreamChannelStatusUpdate(streamID, channelID, ONLINE)
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtsp",
		"call":    "Start",
	}).Debugln("Success connection RTSP")

	// release
	defer func() {
		RTSPClient.Close()
		Storage.StreamChannelStatusUpdate(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	checkClients := time.NewTimer(time.Duration(timeoutClientCheck) * time.Second)
	defer checkClients.Stop()
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	for {
		select {
		// case <-ctx.Done():
		// 	log.WithFields(logrus.Fields{
		// 		"module":  "core",
		// 		"stream":  streamID,
		// 		"channel": channelID,
		// 		"func":    "StreamServerRunStreamRtsp",
		// 		"call":    "ctx.Done()",
		// 	}).Debugln("Stream close by ctx.cannel. ", streamID, channelID)
		// 	return 0, nil
		//Check stream have clients
		// case <-checkClients.C:
		// 	if channel.OnDemand && !Storage.ClientHas(streamID, channelID) {
		// 		log.WithFields(logrus.Fields{
		// 			"module":  "core",
		// 			"stream":  streamID,
		// 			"channel": channelID,
		// 			"func":    "StreamServerRunStreamRtsp",
		// 			"call":    "ClientHas",
		// 		}).Debugln("Stream close has no client. ", streamID, channelID)
		// 		return 0, ErrorStreamNoClients
		// 	}
		// 	checkClients.Reset(time.Duration(timeoutClientCheck) * time.Second)
		case <-checkClients.C:
			cCnt := Storage.ClientCount(streamID, channelID)
			if cCnt == 0 {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "writePktToClient",
					"call":    "ClientCount",
				}).Debugln("Stream close has no client. ", streamID, channelID)
				return 0, ErrorStreamNoClients
			}
			log.Println("clients: ", cCnt)
			checkClients.Reset(time.Duration(timeoutClientCheck) * time.Second)
		//Read core signals
		case signals := <-channel.signals:
			switch signals {
			case SignalStreamStop:
				return 0, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				//TODO:
				// return 0, ErrorStreamRestart
			case SignalStreamCodecUpdate:
				//TODO:
				// return 0, ErrorStreamChannelCodecUpdate
			}
		//Read rtsp signals
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
			case rtspv2.SignalStreamRTPStop:
				return 0, ErrorStreamStopRTSPSignal
			}
		// read rtp.Pkt for cast all clients.
		// case packetRTP := <-RTSPClient.OutgoingProxyQueue:
		// Storage.StreamChannelCastProxy(streamID, channelID, packetRTP)
		// read av.Pkt for cast all clients.
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if packetAV.IsKeyFrame {
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

func StreamServerRunStreamRtmp(ctx context.Context, streamID string, channelID string, channel *ChannelST) (int, error) {
	// rtmp client dial
	RTMPConn, err := rtmp.Dial(channel.URL)
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
		channel.signals <- SignalStreamCodecUpdate
		// log.WithFields(logrus.Fields{
		// 	"module":  "core",
		// 	"stream":  streamID,
		// 	"channel": channelID,
		// 	"func":    "StreamServerRunStreamRtmp",
		// 	"call":    "StreamChannelCodecsUpdate",
		// }).Debugln("RTSPClient.CodecData update send ---> ", JsonFormat(RTSPClient.CodecData))
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtmp",
		"call":    "StreamServerRunStreamRtmp",
	}).Debugln("Success connection RTMP")

	// stream status update
	Storage.StreamChannelStatusUpdate(streamID, channelID, ONLINE)
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamServerRunStreamRtmp",
		"call":    "Start",
	}).Debugln("Success connection RTMP")

	// release
	defer func() {
		RTMPConn.Close()
		Storage.StreamChannelStatusUpdate(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	checkClients := time.NewTimer(time.Duration(timeoutClientCheck) * time.Second)
	defer checkClients.Stop()
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	for {
		select {
		// case <-ctx.Done():
		// 	log.WithFields(logrus.Fields{
		// 		"module":  "core",
		// 		"stream":  streamID,
		// 		"channel": channelID,
		// 		"func":    "StreamServerRunStreamRtmp",
		// 		"call":    "ctx.Done()",
		// 	}).Debugln("Stream close by cannel. ", streamID, channelID)
		// 	return 0, nil
		//Check stream have clients
		case <-checkClients.C:
			if channel.OnDemand && !Storage.ClientHas(streamID, channelID) {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamServerRunStreamRtmp",
					"call":    "ClientHas",
				}).Debugln("Stream close has no client. ", streamID, channelID)
				return 0, ErrorStreamNoClients
			}
			checkClients.Reset(time.Duration(timeoutClientCheck) * time.Second)
		//Read core signals
		case signals := <-channel.signals:
			switch signals {
			case SignalStreamStop:
				return 0, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				//TODO:
				// return 0, ErrorStreamRestart
			case SignalStreamCodecUpdate:
				//TODO:
				// return 0, ErrorStreamChannelCodecUpdate
			}
		//Read av.Pkt,and proxy for all clients.
		// TODO: av.Pkt be save file here.
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

			// send av.RTP
			// Storage.StreamChannelCastProxy(streamID, channelID, &pktRTMP.Data)

			if pktRTMP.IsKeyFrame {
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, pktRTMP.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = pktRTMP.Time
			}
			Seq = append(Seq, &pktRTMP)
			// send av.pkt
			Storage.StreamChannelCast(streamID, channelID, &pktRTMP)

		}
	}
}
