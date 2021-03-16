package gss

import (
	"context"

	"github.com/cgoder/vdk/av"
	log "github.com/sirupsen/logrus"
)

func (svr *ServerST) PlayPre(ctx context.Context, programID string, channelID string) ([]av.CodecData, error) {
	// check stream status
	_, err := svr.ChannelGet(programID, channelID)
	if err != nil {
		log.WithFields(log.Fields{
			"module":  "api",
			"stream":  programID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelGet",
		}).Errorln(ErrorProgramNotFound.Error())
		return nil, ErrorProgramNotFound
	}

	// streaming
	err = svr.ChannelRun(ctx, programID, channelID)
	if err != nil {
		log.WithFields(log.Fields{
			"module":  "api",
			"stream":  programID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelRun",
		}).Errorln(err)
		return nil, ErrorProgramNotFound
	}

	log.WithFields(log.Fields{
		"module":  "api",
		"stream":  programID,
		"channel": channelID,
		"func":    "HTTPAPIServerStreamMSE",
		"call":    "StreamChannelRun",
	}).Debugln("play stream ---> ", programID, channelID)

	// get stream av.Codec
	codecs, err := svr.StreamCodecGet(programID, channelID)
	if err != nil {
		log.WithFields(log.Fields{
			"module":  "http_mse",
			"stream":  programID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return nil, ErrorStreamCodecNotFound
	}

	return codecs, nil
}
