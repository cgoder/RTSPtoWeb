package main

import (
	"time"

	"github.com/deepch/vdk/av"
	"github.com/sirupsen/logrus"
)

//StreamChannelMake check stream exist
func (obj *StorageST) StreamChannelMake(val ChannelST) ChannelST {
	//make client's
	val.clients = make(map[string]ClientST)
	//make last ack
	val.ack = time.Now().Add(-255 * time.Hour)
	//make hls buffer
	val.hlsSegmentBuffer = make(map[int]Segment)
	//make signals buffer chain
	val.signals = make(chan int, 100)
	return val
}

//StreamChannelRunAll run all stream go
func (obj *StorageST) StreamChannelRunAll() {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	for k, v := range obj.Streams {
		for ks, vs := range v.Channels {
			if !vs.OnDemand {
				vs.runLock = true
				go StreamServerRunStreamDo(k, ks)
				v.Channels[ks] = vs
				obj.Streams[k] = v
			}
		}
	}
}

//StreamChannelRun one stream and lock
func (obj *StorageST) StreamChannelRun(streamID string, channelID string) {
	// log.WithFields(logrus.Fields{
	// 	"module":  "http_hls",
	// 	"stream":  streamID,
	// 	"channel": channelID,
	// 	"func":    "StreamChannelRun",
	// 	"call":    "StreamChannelRun",
	// }).Debugln("StreamChannelRun ->>>>>>>>>>>>>>>>>")

	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			if !channelTmp.runLock {
				channelTmp.runLock = true
				streamTmp.Channels[channelID] = channelTmp
				obj.Streams[streamID] = streamTmp
				go StreamServerRunStreamDo(streamID, channelID)
			}
		}
	}
}

//StreamChannelUnlock unlock status to no lock
func (obj *StorageST) StreamChannelUnlock(streamID string, channelID string) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			channelTmp.runLock = false
			streamTmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = streamTmp
		}
	}
}

//StreamChannelControl get stream
func (obj *StorageST) StreamChannelControl(streamID string, channelID string) (*ChannelST, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			return &channelTmp, nil
		}
	}
	return nil, ErrorStreamNotFound
}

//StreamChannelExist check stream exist
func (obj *StorageST) StreamChannelExist(streamID string, channelID string) bool {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			channelTmp.ack = time.Now()
			streamTmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = streamTmp
			return ok
		}
	}
	return false
}

//StreamChannelReload reload stream
func (obj *StorageST) StreamChannelReload(uuid string, channelID string) error {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.signals <- SignalStreamRestart
			return nil
		}
	}
	return ErrorStreamNotFound
}

//StreamInfo return stream info
func (obj *StorageST) StreamChannelInfo(uuid string, channelID string) (*ChannelST, error) {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			return &channelTmp, nil
		}
	}
	return nil, ErrorStreamNotFound
}

//StreamChannelCodecs get stream codec storage or wait
func (obj *StorageST) StreamChannelCodecs(streamID string, channelID string) ([]av.CodecData, error) {
	obj.mutex.RLock()
	tmp, ok := obj.Streams[streamID]
	obj.mutex.RUnlock()
	if !ok {
		return nil, ErrorStreamNotFound
	}
	channelTmp, ok := tmp.Channels[channelID]
	if !ok {
		return nil, ErrorStreamChannelNotFound
	}

	if channelTmp.runLock && channelTmp.codecs != nil {
		// log.WithFields(logrus.Fields{
		// 	"module":  "http_mse",
		// 	"stream":  streamID,
		// 	"channel": channelID,
		// 	"func":    "StreamChannelCodecs",
		// 	"call":    "chan.updated",
		// }).Debugln("Got old codec!")
		return channelTmp.codecs, nil
	}

	t1 := time.Now().UTC()

	timer := time.NewTimer(20 * time.Second)
	for {
		select {
		case <-timer.C:
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamChannelCodecs",
				"call":    "chan.updated",
			}).Errorln("Get codec timeout!")
			return nil, ErrorStreamChannelCodecNotFound
		case <-channelTmp.updated:
			obj.mutex.RLock()
			channelTmp, ok := obj.Streams[streamID].Channels[channelID]
			obj.mutex.RUnlock()
			if !ok {
				return nil, ErrorStreamChannelNotFound
			}

			t2 := time.Now().UTC().Sub(t1)
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamChannelCodecs",
				"call":    "chan.updated",
			}).Debugf("Got Stream codec update! cost:%v", t2.String())

			return channelTmp.codecs, nil
		}
	}

	// for i := 0; i < 100; i++ {
	// 	obj.mutex.RLock()
	// 	tmp, ok := obj.Streams[streamID]
	// 	obj.mutex.RUnlock()
	// 	if !ok {
	// 		return nil, ErrorStreamNotFound
	// 	}
	// 	channelTmp, ok := tmp.Channels[channelID]
	// 	if !ok {
	// 		return nil, ErrorStreamChannelNotFound
	// 	}

	// 	if channelTmp.codecs != nil {
	// 		return channelTmp.codecs, nil
	// 	}
	// 	time.Sleep(50 * time.Millisecond)
	// }
	// return nil, ErrorStreamChannelCodecNotFound
}

//StreamChannelStatus change stream status
func (obj *StorageST) StreamChannelStatus(streamID string, channelID string, val int) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.Status = val
			tmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = tmp
		}
	}
}

//StreamChannelCast broadcast stream
func (obj *StorageST) StreamChannelCast(streamID string, channelID string, val *av.Packet) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			if len(channelTmp.clients) > 0 {
				for _, i2 := range channelTmp.clients {
					if i2.mode == RTSP {
						continue
					}
					if len(i2.outgoingAVPacket) < 1000 {
						i2.outgoingAVPacket <- val
					} else if len(i2.signals) < 10 {
						//send stop signals to client
						i2.signals <- SignalStreamStop
						//No need close socket only send signal to reader / writer socket closed if client go to offline
						/*
							err := i2.socket.Close()
							if err != nil {
								log.WithFields(logrus.Fields{
									"module":  "storage",
									"stream":  streamID,
									"channel": channelID,
									"func":    "CastProxy",
									"call":    "Close",
								}).Errorln(err.Error())
							}
						*/
					}
				}
				channelTmp.ack = time.Now()
				tmp.Channels[channelID] = channelTmp
				obj.Streams[streamID] = tmp
			}
		}
	}
}

//StreamChannelCastProxy broadcast stream
func (obj *StorageST) StreamChannelCastProxy(streamID string, channelID string, val *[]byte) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			if len(channelTmp.clients) > 0 {
				for _, i2 := range channelTmp.clients {
					if i2.mode != RTSP {
						continue
					}
					if len(i2.outgoingRTPPacket) < 1000 {
						i2.outgoingRTPPacket <- val
					} else if len(i2.signals) < 10 {
						//send stop signals to client
						i2.signals <- SignalStreamStop
						err := i2.socket.Close()
						if err != nil {
							log.WithFields(logrus.Fields{
								"module":  "storage",
								"stream":  streamID,
								"channel": channelID,
								"func":    "CastProxy",
								"call":    "Close",
							}).Errorln(err.Error())
						}
					}
				}
				channelTmp.ack = time.Now()
				tmp.Channels[channelID] = channelTmp
				obj.Streams[streamID] = tmp
			}
		}
	}
}

//StreamChannelCodecsUpdate update stream codec storage
func (obj *StorageST) StreamChannelCodecsUpdate(streamID string, channelID string, val []av.CodecData, sdp []byte) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.codecs = val
			channelTmp.sdp = sdp
			tmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = tmp
		}
	}
}

//StreamChannelSDP codec storage or wait
func (obj *StorageST) StreamChannelSDP(streamID string, channelID string) ([]byte, error) {
	// why for 100 times?
	for i := 0; i < 100; i++ {
		obj.mutex.RLock()
		tmp, ok := obj.Streams[streamID]
		obj.mutex.RUnlock()
		if !ok {
			return nil, ErrorStreamNotFound
		}
		channelTmp, ok := tmp.Channels[channelID]
		if !ok {
			return nil, ErrorStreamChannelNotFound
		}

		if len(channelTmp.sdp) > 0 {
			return channelTmp.sdp, nil
		}
		// why sleep?
		time.Sleep(50 * time.Millisecond)
	}
	return nil, ErrorStreamNotFound
}

//StreamChannelAdd add stream
func (obj *StorageST) StreamChannelAdd(uuid string, channelID string, val ChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if _, ok := obj.Streams[uuid]; !ok {
		return ErrorStreamNotFound
	}
	if _, ok := obj.Streams[uuid].Channels[channelID]; ok {
		return ErrorStreamChannelAlreadyExists
	}
	val = obj.StreamChannelMake(val)
	obj.Streams[uuid].Channels[channelID] = val
	if !val.OnDemand {
		val.runLock = true
		go StreamServerRunStreamDo(uuid, channelID)
	}
	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamEdit edit stream
func (obj *StorageST) StreamChannelEdit(uuid string, channelID string, val ChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		if currentChannel, ok := tmp.Channels[channelID]; ok {
			if currentChannel.runLock {
				currentChannel.signals <- SignalStreamStop
			}
			val = obj.StreamChannelMake(val)
			obj.Streams[uuid].Channels[channelID] = val
			if !val.OnDemand {
				val.runLock = true
				go StreamServerRunStreamDo(uuid, channelID)
			}
			err := obj.SaveConfig()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return ErrorStreamNotFound
}

//StreamChannelDelete stream
func (obj *StorageST) StreamChannelDelete(uuid string, channelID string) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			if channelTmp.runLock {
				channelTmp.signals <- SignalStreamStop
			}
			delete(obj.Streams[uuid].Channels, channelID)
			err := obj.SaveConfig()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return ErrorStreamNotFound
}
