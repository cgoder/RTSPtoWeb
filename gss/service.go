package gss

func NewService() *ServerST {
	svr := &ServerST{}
	svr.LoadConfig()
	return svr
}
