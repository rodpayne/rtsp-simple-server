package core

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"

	"github.com/aler9/rtsp-simple-server/internal/conf"
	"github.com/aler9/rtsp-simple-server/internal/logger"
)

func muxHandle(mux *http.ServeMux, method string, path string,
	cb func(w http.ResponseWriter, r *http.Request)) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		cb(w, r)
	})
}

type apiConfigGetRes struct {
	Conf *conf.Conf
	Err  error
}

type apiConfigGetReq struct {
	Res chan apiConfigGetRes
}

type apiPathsItem struct {
	Name        string          `json:"name"`
	ConfName    string          `json:"confName"`
	Conf        *conf.PathConf  `json:"conf"`
	SourceState pathSourceState `json:"sourceState"`
	Source      interface{}     `json:"source"`
}

type apiPathsListData struct {
	Items []apiPathsItem `json:"items"`
}

type apiPathsListRes1 struct {
	Paths map[string]*path
	Err   error
}

type apiPathsListReq1 struct {
	Res chan apiPathsListRes1
}

type apiPathsListRes2 struct {
	Err error
}

type apiPathsListReq2 struct {
	Data *apiPathsListData
	Res  chan apiPathsListRes2
}

type apiParent interface {
	Log(logger.Level, string, ...interface{})
}

type api struct {
	conf        *conf.Conf
	pathManager *pathManager
	parent      apiParent

	mutex sync.Mutex
	s     *http.Server
}

func newAPI(
	address string,
	conf *conf.Conf,
	pathManager *pathManager,
	parent apiParent,
) (*api, error) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	a := &api{
		conf:        conf,
		pathManager: pathManager,
		parent:      parent,
	}

	mux := http.NewServeMux()

	muxHandle(mux, http.MethodGet, "/config/get", a.onConfigGet)
	muxHandle(mux, http.MethodGet, "/paths/list", a.onPathsList)

	// GET /rtspsession/list
	// POST /rtspsession/kick/id
	// GET /rtmpconn/list
	// POST /rtmpconn/kick/id

	a.s = &http.Server{
		Handler: mux,
	}

	go a.s.Serve(ln)

	parent.Log(logger.Info, "[API] listener opened on "+address)

	return a, nil
}

func (a *api) close() {
	a.s.Shutdown(context.Background())
}

func (a *api) onConfigGet(w http.ResponseWriter, r *http.Request) {
	a.mutex.Lock()
	c := a.conf
	a.mutex.Unlock()

	enc, _ := json.Marshal(c)
	w.Write(enc)
}

func (a *api) onPathsList(w http.ResponseWriter, r *http.Request) {
	res := a.pathManager.OnAPIPathsList(apiPathsListReq1{})
	if res.Err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := apiPathsListData{
		Items: []apiPathsItem{},
	}

	for _, pa := range res.Paths {
		pa.OnAPIPathsList(apiPathsListReq2{Data: &data})
	}

	enc, _ := json.Marshal(data)
	w.Write(enc)
}

// OnConfReload is called by core.
func (a *api) OnConfReload(conf *conf.Conf) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.conf = conf
}
