package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/inconshreveable/go-update"
	"github.com/takama/daemon"
	"io"
	"io/fs"
	"main/lib/helpers"
	"main/lib/log"
	"main/lib/structs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/rpc"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service wrap

type AgentServiceWrap struct {
	daemon.Daemon
	tasks []*helpers.GoTask
	lock  *sync.Mutex
	Agent
}

func NewAgentServiceWrap(agent *Agent) *AgentServiceWrap {
	serviceDaemon, err := daemon.New(
		ServiceName,
		ServiceDescription,
		daemon.SystemDaemon,
		ServiceDependencies...,
	)
	if err != nil {
		panic(err)
	}

	return &AgentServiceWrap{
		Daemon: serviceDaemon,
		tasks:  make([]*helpers.GoTask, 0),
		Agent:  *agent,
		lock:   &sync.Mutex{},
	}
}

func (service *AgentServiceWrap) RpcLock(_ int, _ *int) error {
	log.Log.Debug().Msg("RpcLock()")
	can := service.lock.TryLock()
	if !can {
		return errors.New("can't lock, service is busy")
	}
	return nil
}

func (service *AgentServiceWrap) RpcUnlock(_ int, _ *int) error {
	service.FilesReload()
	service.lock.Unlock()
	log.Log.Debug().Msg("RpcUnlock()")
	return nil
}

func (service *AgentServiceWrap) RpcReconfigure(_ int, _ *int) error {
	service.stopTasks()
	service.Settings = LoadSettings()
	service.FilesReload()
	service.startTasks()
	log.Log.Info().Msg("Service was reconfigured")
	return nil
}

func (service *AgentServiceWrap) startTasks() {
	for _, task := range service.tasks {
		go task.Run()
	}
}

func (service *AgentServiceWrap) stopTasks() {
	log.Log.Debug().Msg("stopTasks()")
	for _, task := range service.tasks {
		task.StopChan <- true
	}
	service.tasks = make([]*helpers.GoTask, 0)
}

func (service *AgentServiceWrap) OnServiceStart() {
	err := rpc.RegisterName(ServiceName, service)
	if err != nil {
		panic(err)
	}
	service.tasks = append(service.tasks, helpers.NewAgentTask("HttpServer", 0, func() error {
		handler := http.NewServeMux()
		handler.HandleFunc("/proxy/", service.ProxyRoute())
		handler.HandleFunc("/log", service.LogRoute())
		handler.HandleFunc("/app/check", service.AppCheckRoute())
		handler.HandleFunc("/app/update", service.AppUpdateRoute())
		log.Log.Info().Msgf("Http server started on port %s", service.Settings.HttpPort)
		return http.ListenAndServe(service.Settings.HttpPort, handler)
	}, nil))
	rpcListener, _ := net.Listen("tcp", service.Settings.RpcPort)
	service.tasks = append(service.tasks, helpers.NewAgentTask("RpcServer", 0, func() error {

		log.Log.Info().Msgf("Rpc server started on port %s", service.Settings.RpcPort)
		rpc.Accept(rpcListener)
		return nil
	}, func() {
		err := rpcListener.Close()
		if err != nil {
			return
		}
	}))
	if service.Settings.RemoteCommandsEnabled {
		service.tasks = append(service.tasks,
			helpers.NewAgentTask("AutoFetchCommands",
				time.Duration(service.Settings.CommandsTimeout)*time.Second,
				WithLock(service.lock, func() error {
					cmd := service.ApiClient.GetCommand()
					if cmd != nil {
						service.CommandId = &cmd.ID
						cmdErr := SelectCommand(strings.Split(cmd.Command, " "), &service.Agent)
						service.CommandId = nil
						return cmdErr
					}
					return nil
				},
				), nil,
			),
		)
	}
	service.tasks = append(service.tasks, helpers.NewAgentTask("AutoLogPost", 10*time.Second, func() error {
		buffer := service.FilesGetLogsBuffer()
		if buffer == nil {
			log.Log.Info().Str("Task", "AutoLogPost").Msg("Log buffer is empty, skip task")
			return nil
		}
		if len(buffer.Logs) == 0 {
			log.Log.Info().Str("Task", "AutoLogPost").Msg("Log buffer is empty, skip task")
			return nil
		}
		if service.ApiClient.PostLogData(buffer.Logs) {
			clearErr := service.FilesClearLogsBuffer()
			if clearErr != nil {
				return clearErr
			}
		}
		return nil
	}, nil))
	service.startTasks()
}

func (service *AgentServiceWrap) OnServiceStop() {
	WithLock(service.lock, func() error {
		service.stopTasks()
		return nil
	})
}

func (service *AgentServiceWrap) ProxyRoute() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		u, _ := url.Parse(fmt.Sprintf("http://%s%s",
			service.Settings.NetInfo.ControlIp,
			service.Settings.NetInfo.ControlPort))
		log.Log.Debug().Msgf("Proxy request %s  to %s", req.URL.Path, u.String())
		proxy := httputil.NewSingleHostReverseProxy(u)
		modifyRequest := func(req *http.Request) {
			if !service.Settings.NetInfo.AsProxy {
				req.URL.Path = strings.Split(req.URL.Path, "proxy")[1]
			}
			if strings.Contains(req.URL.Path, "reg") {
				var oldBody structs.RestRegPost
				bodyBytes, _ := io.ReadAll(req.Body)
				httpErr := req.Body.Close()
				if httpErr != nil {
					log.Log.Error().Err(httpErr).Str("Proxy", "body close fails").Msgf(req.URL.String())
					return
				}
				httpErr = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&oldBody)
				if httpErr != nil {
					log.Log.Error().Err(httpErr).Str("Json", "decode fails").Msgf(req.URL.String())
					return
				}
				if oldBody.ProxyInfo != nil {
					maxKey := 0
					for key, _ := range oldBody.ProxyInfo {
						if key > maxKey {
							maxKey = key
						}
					}
					oldBody.ProxyInfo[maxKey+1] = service.Settings.SECRET
				} else {
					oldBody.ProxyInfo = map[int]string{0: service.Settings.SECRET}
				}
				newBody, _ := json.Marshal(oldBody)
				req.Body = io.NopCloser(bytes.NewReader(newBody))
				req.ContentLength = int64(len(newBody))
			}
		}
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			modifyRequest(req)
		}
		proxy.ServeHTTP(w, req)
	}
}

func (service *AgentServiceWrap) LogRoute() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var logData structs.RestLogPost
		bodyBytes, bodyErr := io.ReadAll(req.Body)
		bodyErr = req.Body.Close()
		if bodyErr != nil {
			log.Log.Error().Err(bodyErr).Str("Proxy", "body parse err").Msgf(req.URL.String())
			_ = WriteJsonResponse(w, http.StatusBadRequest, map[string]any{
				"status": "log body parse err",
			})
			return
		}
		mErr := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&logData)
		if mErr != nil {
			_ = WriteJsonResponse(w, http.StatusBadRequest, map[string]any{
				"status": "log body parse err",
			})
			return
		}
		if logData.Context == nil {
			_ = WriteJsonResponse(w, http.StatusBadRequest, map[string]any{
				"status": "context must not be null",
			})
			return
		}
		sendStruct := make([]*structs.RestLogPost, 0)
		sendStruct = append(sendStruct, &logData)
		if !service.ApiClient.PostLogData(sendStruct) {
			_ = WriteJsonResponse(w, http.StatusOK, map[string]any{
				"status": "log saved in buffer",
			})
			service.FilesStoreLogInBuffer(&logData)
		}
	}
}

func (service *AgentServiceWrap) AppCheckRoute() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		params, err := structs.ApplicationUpdateParamsFromReq(req)
		if err != nil {
			httpErr := WriteJsonResponse(w,
				http.StatusBadRequest,
				map[string]any{"msg": errors.New("can't parse request").Error()})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
		}
		exist, _, updErr := service.getApplicationUpdate(params)
		if !*exist {
			httpErr := WriteJsonResponse(w, http.StatusNotFound, map[string]any{
				"msg":    errors.New("update not found").Error(),
				"result": false,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
			return
		}
		if err != nil {
			httpErr := WriteJsonResponse(w, http.StatusConflict, map[string]any{
				"msg":    updErr.Error(),
				"result": false,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
			return
		} else {
			httpErr := WriteJsonResponse(w, http.StatusOK, map[string]any{
				"msg":    "update exists",
				"result": true,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
			return
		}
	}
}

func (service *AgentServiceWrap) AppUpdateRoute() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		params, err := structs.ApplicationUpdateParamsFromReq(req)
		if err != nil {
			httpErr := WriteJsonResponse(w, http.StatusBadRequest, map[string]any{
				"msg":    errors.New("can't parse request").Error(),
				"result": nil,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
		}
		exist, filePath, updErr := service.getApplicationUpdate(params)
		if !*exist {
			httpErr := WriteJsonResponse(w, http.StatusNotFound, map[string]any{
				"msg":    errors.New("update not found"),
				"result": nil,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
		}
		if updErr != nil {
			httpErr := WriteJsonResponse(w, http.StatusConflict, map[string]any{
				"msg":    updErr.Error(),
				"result": nil,
			})
			if httpErr != nil {
				log.Log.Error().Err(httpErr).Msgf(req.URL.String())
				return
			}
		} else {
			http.ServeFile(w, req, *filePath)
		}
	}
}

// Update wrap

type AgentUpdateWrap struct {
	AgentServiceWrap
}

func NewAgentUpdateWrap(agent *Agent) *AgentUpdateWrap {
	serviceWrap := NewAgentServiceWrap(agent)
	return &AgentUpdateWrap{
		AgentServiceWrap: *serviceWrap,
	}
}

func (upd *AgentUpdateWrap) SelfUpdateProcess() {
	msgStop, stopErr := upd.Daemon.Stop()
	if stopErr != nil {
		log.Log.Warn().Err(stopErr).Msgf("Can't stop daemon %s", msgStop)
	}
	updateOptions := update.Options{TargetMode: 0777}
	perErr := updateOptions.CheckPermissions()
	if perErr != nil {
		log.Log.Fatal().Err(perErr).Msg("Can't check permissions to executable file")
		return
	}
	software := upd.ApiClient.GetSelfUpdate()
	if software != nil {
		oldAgentSoftware := *software
		oldAgentSoftware.Version = PcaVersion
		log.Log.Info().Msgf("Update Agent,%s", VersionStringColor(&oldAgentSoftware, software))
		if software.Changelog != nil {
			log.Log.Info().Msgf("ChangeLog: %s", software.Changelog)
		}
		apply := AskConfirm(ForceCmd)
		if !apply {
			return
		}
		progressTrack := *helpers.NewProgressBar(3, 1)
		progressTrack.SetMessageWidth(50)
		go progressTrack.Render()
		upd.downloadSoftware(software, nil, progressTrack)
		upd.unpackBuildFile(software.Build, nil, progressTrack)
		filePath := ""
		_ = filepath.Walk(path.Join(upd.Settings.TmpDir, software.Build.FileSpec.Name),
			func(path string, info fs.FileInfo, err error) error {
				if !info.IsDir() && (strings.Contains("pca", info.Name()) || strings.Contains("main", info.Name())) {
					filePath = path
				}
				return nil
			})
		if filePath == "" {
			log.Log.Warn().Msg("File download succeeded, but no pca file was found in archive")
			return
		}
		reader, err := os.Open(filePath)
		if err != nil {
			log.Log.Fatal().Msg("Failed to open pca file")
			return
		}
		err = update.Apply(reader, updateOptions)
		if err != nil {
			log.Log.Fatal().Msg("Failed to apply update")
			return
		}

		upd.ApiClient.Notify(upd.createNotification("upgraded", map[string]any{"version": software.Version}))
		log.Log.Info().Msgf("Update succeeded, current version: %s", software.Version)
		msg, serviceErr := upd.Daemon.Start()
		if serviceErr != nil {
			log.Log.Warn().Err(serviceErr).Msg("Failed to start daemon")
			return
		}
		log.Log.Info().Msg(msg)
	} else {
		log.Log.Info().Msg("Update for agent not found")
	}
}
