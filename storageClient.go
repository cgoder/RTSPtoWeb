package main

import (
	"github.com/cgoder/vdk/av"
)

var lenAvPacketQueue int = 100
var lenClientSignalQueue int = 100

//ClientAdd Add New Client to Translations
func (obj *StorageST) ClientAdd(streamID string, channelID string, mode int) (string, chan *av.Packet, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return "", nil, ErrorStreamChannelNotFound
	}

	//Generate UUID client
	cid := GenerateUUID()
	chAV := make(chan *av.Packet, lenAvPacketQueue)
	chSignal := make(chan int, lenClientSignalQueue)
	client := &ClientST{UUID: cid, protocol: mode, outgoingAVPacket: chAV, signals: chSignal}

	ch.clients[client.UUID] = client

	// log.WithFields(logrus.Fields{
	// 	"module":  "storageClient",
	// 	"stream":  streamID,
	// 	"channel": channelID,
	// 	"func":    "ClientAdd",
	// 	"call":    "ClientAdd",
	// }).Debugln("client Add ---> ")

	// log.Println(obj)

	return cid, chAV, nil

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
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return false
	}

	if len(ch.clients) > 0 {
		return true
	}
	return false
}

//ClientHas check is client ext
func (obj *StorageST) ClientCount(streamID string, channelID string) int {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return 0
	}

	return len(ch.clients)
}

//ClientCountAll count all clients
func (obj *StorageST) ClientCountAll() int {
	var cnt int
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	for _, st := range obj.Streams {
		for _, ch := range st.Channels {
			cnt = cnt + len(ch.clients)
		}
	}

	return cnt
}
