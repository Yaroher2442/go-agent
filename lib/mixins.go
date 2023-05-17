package lib

import (
	"main/lib/helpers"
	"main/lib/log"
	"main/lib/structs"
	"net/rpc"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// File Watcher

type FilesWatcherMixin struct {
	Settings  *Settings
	Installed []*SavedInfo
	Tmp       *SavedInfo
	CommandId *int
}

type SavedInfo struct {
	Unit    *structs.Unit    `json:"unit" groups:"local"`
	Client  *structs.Client  `json:"client" groups:"local"`
	Product *structs.Product `json:"product" groups:"local"`
	Package *structs.Package `json:"package" groups:"local"`
}

type LogBufferFile struct {
	Logs []*structs.RestLogPost `json:"logs"`
}

func (fw *FilesWatcherMixin) FilesReload() {
	fw.FilesLoadInstalled()
}

func (fw *FilesWatcherMixin) FilesLoadInstalled() {
	files, _ := os.ReadDir(fw.Settings.InfoDir)
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), ".json") {
			installation := &SavedInfo{}
			SafeReadJsonFile(path.Join(fw.Settings.InfoDir, file.Name()), installation)
			fw.Installed = append(fw.Installed, installation)
		}
	}
}

func (fw *FilesWatcherMixin) FilesAddInstallation(client *structs.Client, product *structs.Product, pkg *structs.Package, unit *structs.Unit) {
	fw.Tmp = &SavedInfo{
		Client:  client,
		Product: product,
		Package: pkg,
		Unit:    unit,
	}
}

func (fw *FilesWatcherMixin) FilesForgetPackage(packageId int) {
	fw.Installed = helpers.Filter(fw.Installed, func(info *SavedInfo) bool {
		if info.Package.ID == packageId {
			os.Remove(path.Join(fw.Settings.InfoDir, info.Package.Name+".json"))
			return false
		}
		return true
	})
}

func (fw *FilesWatcherMixin) filesUpdateInnerIndexes() {
	sort.Slice(fw.Installed, func(i, j int) bool {
		return fw.Installed[i].Package.ID < fw.Installed[j].Package.ID
	})
	for i, installed := range fw.Installed {
		installed.Package.InnerIndex = i + 1
	}
}

func (fw *FilesWatcherMixin) FilesIterInstalledPackageItem(iter func(info *SavedInfo, packageItem *structs.PackageItem)) {
	for _, info := range fw.Installed {
		for _, pi := range info.Package.PackageItems {
			iter(info, pi)
		}
	}
}

func (fw *FilesWatcherMixin) FilesIterInstalled(iter func(info *SavedInfo)) {
	for _, info := range fw.Installed {
		iter(info)
	}
}

func (fw *FilesWatcherMixin) FilesGetInfoByPackageID(packageId int) *structs.Package {
	saved := *helpers.Find(fw.Installed, func(i *SavedInfo) bool {
		if i.Package.ID == packageId {
			return true
		}
		return false
	})
	return saved.Package
}

func (fw *FilesWatcherMixin) FilesSerializeInstallation() {
	if fw.Tmp != nil {
		fw.Installed = append(fw.Installed, fw.Tmp)
	}
	fw.filesUpdateInnerIndexes()
	for _, inst := range fw.Installed {
		fileName := path.Join(fw.Settings.InfoDir, inst.Package.Name+".json")
		err := os.WriteFile(fileName, make([]byte, 0), 0666)
		if err != nil {
			continue
		}
		err = SafeWriteJsonFile(inst, JsonLocalOpts, fileName, 0666)
		if err != nil {
			continue
		}
	}
	if !*DEBUG {
		fw.FilesClearTmp()
	}
}

func (fw *FilesWatcherMixin) FilesClearTmp() {
	files, _ := os.ReadDir(fw.Settings.TmpDir)
	for _, file := range files {
		if file.IsDir() {
			os.RemoveAll(path.Join(fw.Settings.TmpDir, file.Name()))
		}
	}
}

func (fw *FilesWatcherMixin) FilesStoreLogInBuffer(logData *structs.RestLogPost) {
	fileName := filepath.Join(fw.Settings.LogDir, "soft.log.json")
	dataBuffer := fw.FilesGetLogsBuffer()
	if dataBuffer == nil {
		return
	}
	dataBuffer.Logs = append(dataBuffer.Logs, logData)
	writeErr := SafeWriteJsonFile(dataBuffer, nil, fileName, 0666)
	if writeErr != nil {
		return
	}
	return
}

func (fw *FilesWatcherMixin) FilesGetLogsBuffer() *LogBufferFile {
	fileName := filepath.Join(fw.Settings.LogDir, "soft.log.json")
	dataBuffer := &LogBufferFile{Logs: make([]*structs.RestLogPost, 0)}
	readErr := SafeReadJsonFile(fileName, &dataBuffer)
	if readErr != nil {
		log.Log.Warn().Err(readErr).Msg("tst")
		return nil
	}
	return dataBuffer
}

func (fw *FilesWatcherMixin) FilesClearLogsBuffer() error {
	fileName := filepath.Join(fw.Settings.LogDir, "soft.log.json")
	dataBuffer := LogBufferFile{Logs: make([]*structs.RestLogPost, 0)}
	err := SafeWriteJsonFile(dataBuffer, nil, fileName, 0666)
	if err != nil {
		return err
	}
	return nil
}

// RPC Client

type RpcClientMixin struct {
	RpcClient *rpc.Client
}

func (rpc *RpcClientMixin) RemoteLock() {
	if rpc.RpcClient != nil {
		var status int
		err := rpc.RpcClient.Call(ServiceName+".RpcLock", status, &status)
		if err != nil {
			rpc.RemoteUnlock()
			log.Log.Fatal().Err(err).Msg("remoteLock() fails")
		}
	}
}

func (rpc *RpcClientMixin) RemoteUnlock() {
	var status int
	if rpc.RpcClient != nil {
		err := rpc.RpcClient.Call(ServiceName+".RpcUnlock", &status, &status)
		if err != nil {
			log.Log.Fatal().Err(err).Msg("remoteUnlock() fails")
		}
	}
}

func (rpc *RpcClientMixin) WithRemoteLock(f func()) {
	rpc.RemoteLock()
	defer rpc.RemoteUnlock()
	f()
	return
}

func (rpc *RpcClientMixin) RemoteReconfigure() {
	if rpc.RpcClient != nil {
		var status int
		err := rpc.RpcClient.Call("Agent.RpcReconfigure", &status, &status)
		if err != nil {
			rpc.RemoteUnlock()
			log.Log.Fatal().Err(err).Msg("RpcReconfigure failed")
		}
	}
}
