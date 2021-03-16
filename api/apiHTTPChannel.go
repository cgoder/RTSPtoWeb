package api

import (
	"context"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

//HTTPAPIServerStreamChannelCodec function return codec info struct
func HTTPAPIServerStreamChannelCodec(c *gin.Context) {
	if !service.ChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: service.ErrorProgramNotFound.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelCodec",
			"call":    "StreamChannelExist",
		}).Errorln(service.ErrorProgramNotFound.Error())
		return
	}
	codecs, err := service.StreamCodecGet(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelCodec",
			"call":    "StreamChannelCodec",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: codecs})
}

//HTTPAPIServerStreamChannelInfo function return stream info struct
func HTTPAPIServerStreamChannelInfo(c *gin.Context) {
	info, err := service.StreamChannelGet(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelInfo",
			"call":    "StreamChannelInfo",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: info})
}

//HTTPAPIServerStreamChannelReload function reload stream
func HTTPAPIServerStreamChannelReload(c *gin.Context) {
	err := service.ChannelReload(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelReload",
			"call":    "StreamChannelReload",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: service.Success})
}

//HTTPAPIServerStreamChannelEdit function edit stream
func HTTPAPIServerStreamChannelEdit(c *gin.Context) {
	var ch service.ChannelST
	err := c.BindJSON(&ch)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamEdit",
			"call":    "BindJSON",
		}).Errorln(err.Error())
		return
	}
	err = service.StreamChannelEdit(context.TODO(), c.Param("uuid"), c.Param("channel"), &ch)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelEdit",
			"call":    "StreamChannelEdit",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: service.Success})
}

//HTTPAPIServerStreamChannelDelete function delete stream
func HTTPAPIServerStreamChannelDelete(c *gin.Context) {
	err := service.StreamChannelDelete(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelDelete",
			"call":    "StreamChannelDelete",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: service.Success})
}

//HTTPAPIServerStreamChannelAdd function add new stream
func HTTPAPIServerStreamChannelAdd(c *gin.Context) {
	var ch service.ChannelST
	err := c.BindJSON(&ch)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelAdd",
			"call":    "BindJSON",
		}).Errorln(err.Error())
		return
	}
	err = service.StreamChannelAdd(context.TODO(), c.Param("uuid"), c.Param("channel"), &ch)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(log.Fields{
			"module":  "http_stream",
			"stream":  c.Param("uuid"),
			"channel": c.Param("channel"),
			"func":    "HTTPAPIServerStreamChannelAdd",
			"call":    "StreamChannelAdd",
		}).Errorln(err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: service.Success})
}
