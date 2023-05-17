package install

import "main/lib/structs"

type Installer struct {
	InstalledPackages []structs.LinuxInstallCandidate
	PackageManager    string
	Flags             string
}

func NewInstaller(manager string, flags string) *Installer {
	return &Installer{
		PackageManager:    manager,
		InstalledPackages: make([]structs.LinuxInstallCandidate, 0),
		Flags:             flags,
	}
}

func (i *Installer) createPackage(name *string, kind string) structs.LinuxInstallCandidate {
	var pkg structs.LinuxInstallCandidate = nil
	switch kind {
	case "application":
		pkg = &AppPackage{}
	default:
		switch i.PackageManager {
		case "apt":
			pkg = &DebPackage{SysPackage: structs.SysPackage{PackageManagerFlags: i.Flags}}
		}
	}
	if name != nil {
		pkg.SetName(*name)
	}
	return pkg
}

func (i *Installer) AddInstalledPackage(kind string, name string) {
	i.InstalledPackages = append(i.InstalledPackages, i.createPackage(&name, kind))
}

func (i *Installer) InstallPackage(kind string, paths ...string) (*structs.Installation, error) {
	var pkg structs.LinuxInstallCandidate
	pkg = i.createPackage(nil, kind)
	installInfo := pkg.ParsePackage(paths...)
	err := pkg.Install(paths...)
	i.InstalledPackages = append(i.InstalledPackages, pkg)
	return installInfo, err
}

func (i *Installer) RollbackAll() error {
	var err error = nil
	for _, installed := range i.InstalledPackages {
		err = installed.Rollback()
	}
	return err
}
