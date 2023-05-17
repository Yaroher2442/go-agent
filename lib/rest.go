package lib

import (
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/websocket"
	"main/lib/log"
	"main/lib/structs"
	"net/http"
	"strconv"
	"strings"
)

var DisplayTraceDebug = false

type RestClient struct {
	settings *Settings
	client   *resty.Client
	Host     string
}

func NewRestClient(settings *Settings) *RestClient {
	host := settings.NetInfo.ControlIp + settings.NetInfo.ControlPort
	client := &RestClient{settings: settings, client: resty.New(), Host: host}
	client.client.SetAuthToken(settings.SECRET)
	client.client.SetBaseURL(settings.NetInfo.Protocol + "://" + host)
	client.client.SetOutputDirectory(settings.TmpDir)
	return client
}

// Helpers

func (rest *RestClient) handleResponseInfo(response *resty.Response, err error) bool {
	if err != nil {
		log.Log.Error().Err(err).Msg("")
		return err != nil
	}
	log.Log.Debug().Str("url", response.Request.Method+" "+response.Request.URL).Msg(response.Status())
	if response.StatusCode() >= 400 {
		switch response.StatusCode() {
		case 409:
			q := response.Error().(*structs.ApiInconsistencyContext)
			log.Log.Error().Msgf("Request is inconsistency %s %s", q.Description, q.ErrorCtx.Comment)
		case 401:
			q := response.Error().(*structs.ApiInconsistencyContext)
			log.Log.Error().Msgf("Agent must be register before operations (pca reg) %s %s", q.Description, q.ErrorCtx.Comment)
		default:
			log.Log.Error().Int("status", response.StatusCode()).Msgf("Can't complete request %s", response.Error())
		}
		return true
	}
	if DisplayTraceDebug {
		log.Log.Debug().Msgf("  Body       :\n", response)
		log.Log.Debug().Msgf("  Time       :", response.Time())
		log.Log.Debug().Msgf("  Proto      :", response.Proto())
		log.Log.Debug().Msgf("  Received At:", response.ReceivedAt())
		ti := response.Request.TraceInfo()
		log.Log.Debug().Msgf("  DNSLookup     :", ti.DNSLookup)
		log.Log.Debug().Msgf("  ConnTime      :", ti.ConnTime)
		log.Log.Debug().Msgf("  TCPConnTime   :", ti.TCPConnTime)
		log.Log.Debug().Msgf("  TLSHandshake  :", ti.TLSHandshake)
		log.Log.Debug().Msgf("  ServerTime    :", ti.ServerTime)
		log.Log.Debug().Msgf("  ResponseTime  :", ti.ResponseTime)
		log.Log.Debug().Msgf("  TotalTime     :", ti.TotalTime)
		log.Log.Debug().Msgf("  IsConnReused  :", ti.IsConnReused)
		log.Log.Debug().Msgf("  IsConnWasIdle :", ti.IsConnWasIdle)
		log.Log.Debug().Msgf("  ConnIdleTime  :", ti.ConnIdleTime)
		log.Log.Debug().Msgf("  RequestAttempt:", ti.RequestAttempt)
	}
	return false

}

func (rest *RestClient) prxRoute(route string) string {
	if rest.settings.NetInfo.AsProxy {
		return fmt.Sprintf("/proxy%s", route)
	}
	return route
}

// SelfApiCalls

func (rest *RestClient) GetSelfUpdate() *structs.Software {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.Software{})
	resp, err := req.Get(rest.prxRoute("/api/v1/agent/update"))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	if resp.String() == "null" {
		return nil
	}
	return resp.Result().(*structs.Software)
}

func (rest *RestClient) DownloadSelfUpdate() *structs.Software {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.RestSoftwareConfigGet{})
	resp, err := req.Get(rest.prxRoute("/api/v1/agent/update"))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	if resp.String() == "null" {
		return nil
	}
	return resp.Result().(*structs.Software)
}

// ApiCalls

func (rest *RestClient) PingServer() bool {
	resp, err := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		Post(rest.prxRoute("/api/v1/agent/ping"))
	return !rest.handleResponseInfo(resp, err)
}

func (rest *RestClient) Reg(dto *structs.RestRegPost) bool {
	resp, err := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetBody(dto).
		Post(rest.prxRoute("/api/v1/agent/reg"))
	return !rest.handleResponseInfo(resp, err)
}

func (rest *RestClient) GetAllClients() []*structs.Client {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.ApiAccessWithTotal[structs.Client]{})
	resp, err := req.Get(rest.prxRoute("/api/v1/agent/client"))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.ApiAccessWithTotal[structs.Client]).Items
}

func (rest *RestClient) GetAllProductsInClient(clientId int) []*structs.Product {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.ApiAccessWithTotal[structs.Product]{})
	resp, err := req.Get(rest.prxRoute(fmt.Sprintf("/api/v1/agent/client/%d/product", clientId)))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.ApiAccessWithTotal[structs.Product]).Items
}

func (rest *RestClient) GetAllUnitsInProduct(prId int, limit int) []*structs.Unit {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetQueryParam("limit", strconv.Itoa(limit)).
		SetResult(&structs.ApiAccessWithTotal[structs.Unit]{})
	resp, err := req.Get(rest.prxRoute(fmt.Sprintf("/api/v1/agent/product/%d/unit", prId)))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.ApiAccessWithTotal[structs.Unit]).Items
}

func (rest *RestClient) GetSoftwareUpdates(unitId int) []*structs.Package {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.ApiAccessWithTotal[structs.Package]{})
	resp, err := req.Get(rest.prxRoute(fmt.Sprintf("/api/v1/agent/unit/%d/update", unitId)))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.ApiAccessWithTotal[structs.Package]).Items
}

func (rest *RestClient) GetSoftwareLatestPatch(softwareId int) *structs.Software {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.Software{})
	resp, err := req.Get(rest.prxRoute(fmt.Sprintf("/api/v1/agent/software/%d/new_patch", softwareId)))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	if resp.String() == "null" {
		return nil
	}
	return resp.Result().(*structs.Software)
}

func (rest *RestClient) unpackHeaders(head http.Header, to *structs.HttpFileInfo) *structs.HttpFileInfo {
	to.Range = structs.FileRangeFromHeader(head.Get("Content-Range"))
	to.Digest = structs.FileDigestFromHeader(head.Get("Digest"))
	to.Disposition = structs.FileDispositionFromHeader(head.Get("Content-Disposition"))
	to.ContentType = structs.FileContentTypeFromHeader(head.Get("content-type"))
	return to
}

func (rest *RestClient) DownloadBuildHEAD(build int, startBy int) *structs.HttpFileSpec {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetQueryParam("start_by", strconv.Itoa(startBy))
	resp, err := req.Head(fmt.Sprintf(rest.prxRoute("/api/v1/agent/build/%d/download"), build))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return rest.unpackHeaders(resp.Header(), &structs.HttpFileInfo{}).ToSpec()
}

func (rest *RestClient) DownloadBuild(buildId int, info *structs.BuildInfo) error {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetQueryParam("start_by", strconv.Itoa(int(info.LoadedBytes))).
		SetOutput(info.Name + "." + info.HttpInfo.FileType)
	resp, err := req.Get(fmt.Sprintf(rest.prxRoute("/api/v1/agent/build/%d/download"), buildId))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return errors.New("download failed")
	}
	info.HttpInfo = rest.unpackHeaders(resp.Header(), &structs.HttpFileInfo{}).ToSpec()
	return nil
}

func (rest *RestClient) GetSoftwareConfig(softwareId int) *structs.RestSoftwareConfigGet {
	req := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetResult(&structs.RestSoftwareConfigGet{})
	resp, err := req.Get(rest.prxRoute(fmt.Sprintf("/api/v1/agent/software/%d/config", softwareId)))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.RestSoftwareConfigGet)
}

func (rest *RestClient) Notify(notification *structs.RestNotifyPost) {
	resp, err := rest.client.R().
		SetAuthToken(rest.settings.SECRET).
		SetResult(&structs.RestSessionGet{}).
		SetBody(notification).
		Post(rest.prxRoute("/api/v1/agent/notify"))
	rest.handleResponseInfo(resp, err)
}

func (rest *RestClient) GetCommand() *structs.RestCommandGet {
	resp, err := rest.client.R().
		SetError(&structs.ApiInconsistencyContext{}).
		SetAuthToken(rest.settings.SECRET).
		SetResult(&structs.RestCommandGet{}).
		Get(rest.prxRoute("/api/v1/agent/cmd"))
	fail := rest.handleResponseInfo(resp, err)
	if fail {
		return nil
	}
	return resp.Result().(*structs.RestCommandGet)
}

// Future

func (rest *RestClient) PostLogData(logData []*structs.RestLogPost) bool {
	resp, err := rest.client.R().
		SetAuthToken(rest.settings.SECRET).
		SetResult(&structs.RestSessionGet{}).
		SetBody(logData).
		Post(rest.prxRoute("/api/v1/agent/log"))
	return !rest.handleResponseInfo(resp, err)
}

func (rest *RestClient) OpenWebsocket() (*websocket.Conn, error) {
	wsProtocol := rest.settings.NetInfo.Protocol
	wsProtocol = strings.Replace(wsProtocol, "http", "ws", 1)
	conf, err := websocket.NewConfig(
		wsProtocol+"://"+rest.Host+rest.prxRoute("/api/v1/agent/shell/open"),
		rest.settings.NetInfo.Protocol+"://"+rest.Host+rest.prxRoute("/api/v1/agent/shell/open"))
	log.Log.Debug().Msgf("%s", conf)
	conf.Header.Set("Authorization", "Bearer "+rest.settings.SECRET)
	client, err := websocket.DialConfig(conf)
	if err != nil {
		return nil, err
	}
	return client, err
}
