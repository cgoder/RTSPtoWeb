package main

import (
	"time"

	"github.com/deepch/vdk/av"
)

var lenAvPacketQueue int = 200
var lenClientSignalQueue int = 100

//ClientAdd Add New Client to Translations
func (obj *StorageST) ClientAdd(streamID string, channelID string, mode int) (string, chan *av.Packet, chan *[]byte, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	streamTmp, ok := obj.Streams[streamID]
	if !ok {
		return "", nil, nil, ErrorStreamNotFound
	}
	//Generate UUID client
	cid, err := generateUUID()
	if err != nil {
		return "", nil, nil, err
	}
	chAV := make(chan *av.Packet, lenAvPacketQueue)
	chRTP := make(chan *[]byte, lenAvPacketQueue)
	channelTmp, ok := streamTmp.Channels[channelID]
	if !ok {
		return "", nil, nil, ErrorStreamNotFound
	}

	channelTmp.clients[cid] = &ClientST{mode: mode, outgoingAVPacket: chAV, outgoingRTPPacket: chRTP, signals: make(chan int, lenClientSignalQueue)}
	channelTmp.ack = time.Now()
	streamTmp.Channels[channelID] = channelTmp
	obj.Streams[streamID] = streamTmp

	// log.WithFields(logrus.Fields{
	// 	"module":  "storageClient",
	// 	"stream":  streamID,
	// 	"channel": channelID,
	// 	"func":    "ClientAdd",
	// 	"call":    "ClientAdd",
	// }).Debugln("client Add ---> ")

	// log.Println(obj)

	return cid, chAV, chRTP, nil

}

//ClientDelete Delete Client
func (obj *StorageST) ClientDelete(streamID string, cid string, channelID string) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if _, ok := obj.Streams[streamID]; ok {
		delete(obj.Streams[streamID].Channels[channelID].clients, cid)
	}
}

//ClientHas check is client ext
func (obj *StorageST) ClientHas(streamID string, channelID string) bool {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	streamTmp, ok := obj.Streams[streamID]
	if !ok {
		return false
	}
	channelTmp, ok := streamTmp.Channels[channelID]
	if !ok {
		return false
	}
	// what is mean? client set 30 seconds auto-offline?
	// if time.Now().Sub(channelTmp.ack).Seconds() > 30 {
	// 	return false
	// }
	if len(channelTmp.clients) > 0 {
		return true
	}
	return true
}

//ClientHas check is client ext
func (obj *StorageST) ClientCount(streamID string, channelID string) int {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	// streamTmp, ok := obj.Streams[streamID]
	// if !ok {
	// 	return 0
	// }
	// channelTmp, ok := streamTmp.Channels[channelID]
	// if !ok {
	// 	return 0
	// }

	return len(obj.Streams[streamID].Channels[channelID].clients)
}
