package install

import (
	"fmt"
	"github.com/pkg/errors"
	"main/lib/helpers"
	"main/lib/log"
	"main/lib/structs"
	"path/filepath"
	"strings"
)

type DebPackage struct {
	structs.SysPackage
}

func (d *DebPackage) Install(paths ...string) error {
	path := paths[0]
	command := fmt.Sprintf(
		"apt-get install %s %s",
		d.PackageManagerFlags,
		path,
	)
	log.Log.Info().Str("command", command).Msg("Execute ")
	errStrings, err := helpers.ShellOutCaptureErr(command)
	if strings.Contains(errStrings, "debconf") {
		log.Log.Debug().Err(errors.New(errStrings)).Msg("debconf error")
		return nil
	}
	if errStrings != "" {
		err = errors.New(fmt.Sprintf("Err String:%s", errStrings))
	}
	if err != nil {
		log.Log.Error().Err(err).Str("output", errStrings).Msgf("Install(%s) failed", path)
	}
	return err
}

func (d *DebPackage) ParsePackage(paths ...string) *structs.Installation {
	filePath := paths[0]
	d.Installation = structs.Installation{}
	if filepath.Ext(filePath) != ".deb" {
		log.Log.Error().Str("func", "DEB::ParsePackage").Str("path", filePath).Msg("File not deb package")
		return &d.Installation
	}
	if !helpers.FileExists(filePath) {
		log.Log.Error().Str("func", "DEB::ParsePackage").Str("path", filePath).Msg("File not found")
		return &d.Installation
	}
	debInfo, _, err := helpers.ShellOutCaptureOutErr(fmt.Sprintf("dpkg-deb -f %s", filePath))
	if err != nil {
		log.Log.Warn().Msg("dpkg-deb parse package failed")
		return &d.Installation
	}
	d.Installation.ControlInfo = &structs.PackageControlInfo{}
	for _, line := range strings.Split(debInfo, "\n") {
		log.Log.Debug().Str("line", line).Msg("Scanned in info")
		switch {
		case strings.Contains(line, "Package:"):
			d.Installation.ControlInfo.PackageName = strings.Split(line, ": ")[1]
			d.Name = d.Installation.ControlInfo.PackageName
		case strings.Contains(line, "Version:"):
			d.Installation.ControlInfo.Version = strings.Split(line, ": ")[1]
		case strings.Contains(line, "Maintainer:"):
			d.Installation.ControlInfo.Maintainer = strings.Split(line, ": ")[1]
		case strings.Contains(line, "Description:"):
			d.Installation.ControlInfo.Description = strings.Split(line, ": ")[1]
		case strings.Contains(line, "Depends:"):
			deps := strings.Split(line, ": ")[1]
			d.Installation.ControlInfo.Depends = strings.Split(deps, ", ")
		}
	}

	debFiles, _, err := helpers.ShellOutCaptureOutErr(fmt.Sprintf("dpkg-deb -c %s", filePath))
	if err != nil {
		log.Log.Warn().Msg("dpkg-deb parse package failed")
		return &d.Installation
	}
	d.Installation.ServiceInfo = &structs.ServiceInfo{}
	for _, line := range strings.Split(debFiles, "\n") {
		log.Log.Debug().Str("line", line).Msg("Scanned in files")
		switch {
		case strings.Contains(line, ".service"):
			serviceName := filepath.Base(line)
			log.Log.Debug().Str("file", serviceName).Msg("Scanned service file")
			d.Installation.ServiceInfo = &structs.ServiceInfo{Name: serviceName}
		}
	}
	return &d.Installation
}

func (d *DebPackage) simulate(path string) error {
	command := fmt.Sprintf(
		"apt-get install --simulate %s %s",
		d.PackageManagerFlags,
		path,
	)
	log.Log.Info().Str("command", command).Msg("Execute ")
	errStrings, err := helpers.ShellOutCaptureErr(command)
	if errStrings != "" {
	}
	if err != nil {
		log.Log.Error().Err(err).Str("output", errStrings).Msgf("Simulate(%s) failed", path)
	}
	return err
}

func (d *DebPackage) Rollback() error {
	command := fmt.Sprintf(
		"DEBIAN_FRONTEND=noninteractive apt-get purge %s %s",
		d.PackageManagerFlags,
		d.Name,
	)
	log.Log.Info().Str("command", command).Msg("Execute ")
	passWarn := func(warn string, pass string) bool {
		return strings.Contains(warn, pass)
	}
	errStr, err := helpers.ShellOutCaptureErr(command)
	if passWarn(errStr, "isn't installed") || passWarn(errStr, "Unable to locate") {
		log.Log.Warn().Str("output", errStr).Msgf("Rollback(%s) skipped", d.Name)
		return nil
	}
	if errStr != "" {
		err = errors.New(errStr)
	}
	if err != nil {
		log.Log.Error().Err(err).Str("output", errStr).Msgf("Rollback(%s) failed", d.Name)
	}
	return err
}
