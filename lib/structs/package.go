package structs

import (
	"github.com/takama/daemon"
	"main/lib/log"
	"strings"
)

// ------------------------------------------------------------

type PackageControlInfo struct {
	PackageName string   `json:"name" groups:"local"`
	Version     string   `json:"version" groups:"local"`
	Maintainer  string   `json:"maintainer" groups:"local"`
	Depends     []string `json:"depends" groups:"local"`
	Description string   `json:"description" groups:"local"`
}

type ServiceInfo struct {
	Name string `json:"name" groups:"local"`
}

func (i *ServiceInfo) Restart() {
	serviceDaemon, _ := daemon.New(
		strings.Split(i.Name, ".")[0],
		"",
		daemon.SystemDaemon,
		[]string{}...,
	)
	stop, err := serviceDaemon.Stop()
	if err != nil {
		log.Log.Debug().Err(err).Msg("Error stopping service")
		if !strings.Contains(err.Error(), "already") {
			return
		}
	} else {
		log.Log.Debug().Msgf("Service %s stopped:\n%s", i.Name, stop)
	}
	start, err := serviceDaemon.Start()
	if err != nil {
		log.Log.Debug().Err(err).Msg("Error starting service")
		return
	}
	log.Log.Debug().Msgf("Service %s started:\n%s", i.Name, start)

}

type AutorunControlInfo struct {
	Path string `json:"path" groups:"local"`
}

type AppInfo struct {
	AppPath string `json:"app_path" groups:"local"`
}

func (i *AutorunControlInfo) Restart() {
}

type Installation struct {
	ControlInfo *PackageControlInfo `json:"control_info" groups:"local"`
	ServiceInfo *ServiceInfo        `json:"service_info" groups:"local"`
	AutorunInfo *AutorunControlInfo `json:"autorun_info" groups:"local"`
	AppInfo     *AppInfo            `json:"app_info" groups:"local"`
}

type LinuxInstallCandidate interface {
	Install(paths ...string) error
	ParsePackage(paths ...string) *Installation
	Rollback() error
	SetName(name string)
}

type SysPackage struct {
	Name string `json:"name" groups:"local"`
	Installation
	PackageManagerFlags string `json:"package_manager_flags" groups:"local"`
}

func (d *SysPackage) Install(paths ...string) error {
	return nil
}

func (d *SysPackage) Rollback() error {
	return nil
}

func (d *SysPackage) ParsePackage(paths ...string) *Installation {
	return nil
}

func (d *SysPackage) SetName(name string) {
	d.Name = name
}

// ------------------------------------------------------------
