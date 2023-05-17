package install

import (
	cp "github.com/otiai10/copy"
	"main/lib/structs"
	"os"
)

type AppPackage struct {
	structs.SysPackage
}

func (d *AppPackage) Install(paths ...string) error {
	src := paths[0]
	dest := paths[1]
	return cp.Copy(src, dest)
}

func (d *AppPackage) ParsePackage(paths ...string) *structs.Installation {
	d.Installation = structs.Installation{AppInfo: &structs.AppInfo{AppPath: paths[1]}}
	d.Name = d.Installation.AppInfo.AppPath
	return &d.Installation
}

func (d *AppPackage) Rollback() error {
	return os.RemoveAll(d.Name)
}
