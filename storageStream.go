package main

import "context"

//StreamsList list all stream
func (obj *StorageST) StreamsList() map[string]*ProgramST {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	tmp := make(map[string]*ProgramST)
	for i, i2 := range obj.Streams {
		tmp[i] = i2
	}
	return tmp
}

//StreamAdd add stream
func (obj *StorageST) StreamAdd(uuid string, val *ProgramST) error {
	progID := uuid

	obj.mutex.Lock()
	_, ok := obj.Streams[progID]
	obj.mutex.Unlock()
	if ok {
		return ErrorStreamAlreadyExists
	}

	obj.StreamChannelStop(progID, "")

	obj.mutex.Lock()
	obj.Streams[progID] = val
	obj.mutex.Unlock()

	for chID, i2 := range val.Channels {
		if !i2.OnDemand {
			// go StreamServerRunStreamDo(context.TODO(), uuid, i)
			go StreamChannelRun(context.TODO(), progID, chID)
		}
	}

	obj.Streams[progID] = val
	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamEdit edit stream
func (obj *StorageST) StreamEdit(uuid string, val *ProgramST) error {
	progID := uuid

	obj.mutex.Lock()
	_, ok := obj.Streams[progID]
	obj.mutex.Unlock()
	if !ok {
		return ErrorStreamNotFound
	}

	obj.StreamChannelStop(progID, "")

	obj.mutex.Lock()
	obj.Streams[progID] = val
	obj.mutex.Unlock()

	for _, ch := range val.Channels {
		if !ch.OnDemand {
			// go StreamServerRunStreamDo(ctx, streamID, channelID)
			go StreamChannelRun(context.Background(), progID, ch.UUID)
		}
	}

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamReload reload stream
func (obj *StorageST) StopAll() {
	obj.StreamChannelStop("", "")
}

//StreamReload reload stream
func (obj *StorageST) StreamReload(uuid string) error {
	// obj.mutex.RLock()
	// defer obj.mutex.RUnlock()
	// if tmp, ok := obj.Streams[uuid]; ok {
	// 	for _, i2 := range tmp.Channels {
	// 		if i2.Status == ONLINE {
	// 			i2.signals <- SignalStreamRestart
	// 		}
	// 	}
	// 	return nil
	// }
	// return ErrorStreamNotFound
	return nil
}

//StreamDelete stream
func (obj *StorageST) StreamDelete(uuid string) error {
	progID := uuid

	obj.mutex.Lock()
	_, ok := obj.Streams[progID]
	obj.mutex.Unlock()
	if !ok {
		return ErrorStreamNotFound
	}

	obj.mutex.Lock()
	delete(obj.Streams, uuid)
	obj.mutex.Unlock()

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamInfo return stream info
func (obj *StorageST) StreamInfo(uuid string) (*ProgramST, error) {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		return tmp, nil
	}
	return nil, ErrorStreamNotFound
}
