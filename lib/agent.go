package lib

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"io"
	"io/fs"
	"main/lib/helpers"
	"main/lib/install"
	"main/lib/log"
	"main/lib/structs"
	"net/rpc"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Agent struct {
	RpcClientMixin
	FilesWatcherMixin
	ApiClient *RestClient
}

func NewAgent(settings *Settings, apiClient *RestClient, rpcClient *rpc.Client) *Agent {

	agent := &Agent{
		RpcClientMixin:    RpcClientMixin{RpcClient: rpcClient},
		ApiClient:         apiClient,
		FilesWatcherMixin: FilesWatcherMixin{Settings: settings},
	}
	agent.FilesLoadInstalled()
	return agent
}

// HELPERS

func (a *Agent) DisplayInstalled() {
	rows := make([][]table.Row, 0)
	head := table.Row{"#",
		"Package",
		"Software",
		//"Version",
		"Kind",
		"Status",
	}
	if *listCmdVerbose {
		head = append(head, "PackageName", "SystemdName", "ServiceStatus")
	}
	t := helpers.ConstructTable(&head)
	a.FilesIterInstalled(func(info *SavedInfo) {
		pRows := make([]table.Row, 0)
		t.AppendRow(table.Row{info.Package.InnerIndex, info.Package.Name})
		for _, pi := range info.Package.PackageItems {
			status := text.FgGreen.Sprint(pi.Software.Build.FileSpec.Status)
			if pi.Software.Build.FileSpec.Status != "Installed" {
				status = text.FgRed.Sprint(pi.Software.Build.FileSpec.Status)
			}
			row := table.Row{"", "",
				fmt.Sprintf("name:%s\nid:%d %s", pi.Software.Name, pi.Software.ID,
					fmt.Sprintf("%s_%s+%d", pi.Software.Branch, pi.Software.Version, pi.Software.Patch)),
				ColorKind(pi.Software.Kind), status}
			if *listCmdVerbose {
				serviceName, serviceStatus := pi.Software.GetServiceInfo()
				row = append(row, pi.Software.GetSysPackageName(),
					serviceName,
					serviceStatus)
			}
			t.AppendRow(row)
		}
		rows = append(rows, pRows)
		t.AppendSeparator()
	})
	t.Render()
}

func (a *Agent) downloadSoftware(software *structs.Software,
	count *helpers.WaitGroupCount,
	progressTrack progress.Writer) {

	// goLoad - download build files
	goLoad := func(build *structs.Build) {
		if count != nil {
			defer count.Done()
		}

		// Track file upload and update progress bar {{
		fileTracker := helpers.NewTracker(int(build.FileSpec.HttpInfo.Size),
			&progress.UnitsBytes,
			nil,
			fmt.Sprintf("Download %s", build.FileSpec.Name))
		progressTrack.AppendTracker(fileTracker)

		filePath := path.Join(a.Settings.TmpDir, build.FileSpec.Name+"."+build.FileSpec.HttpInfo.FileType)
		go func() {
			for {
				if !helpers.FileExists(filePath) {
					continue
				}
				fileState, _ := os.Stat(filePath)
				size := fileState.Size()
				fileTracker.SetValue(size)
				build.FileSpec.LoadedBytes = size
				if size == build.FileSpec.HttpInfo.Size {
					fileTracker.UpdateMessage(text.FgGreen.Sprintf(fileTracker.Message))
					fileTracker.SetValue(size)
					build.FileSpec.LoadedBytes = size
					build.FileSpec.Status = structs.Downloaded
					fileTracker.UpdateMessage(text.FgGreen.Sprintf("Download %s done", build.FileSpec.Name))
					fileTracker.MarkAsDone()
					break
				}
				time.Sleep(50)
			}
		}()
		// }}

		err := a.ApiClient.DownloadBuild(build.ID, build.FileSpec)
		if err != nil {
			build.FileSpec.Status = structs.Errored
			build.FileSpec.Error = err.Error()
			fileTracker.UpdateMessage(text.FgRed.Sprint(fileTracker.Message))
			fileTracker.MarkAsErrored()
		}
	}

	head := a.ApiClient.DownloadBuildHEAD(software.Build.ID, 0)
	software.Build.FileSpec = structs.NewBuildInfo(software.StringWithName(), head)
	if count != nil {
		go goLoad(software.Build)
	} else {
		goLoad(software.Build)
	}
}

func (a *Agent) validateBuildCheckSum(build *structs.Build, count *helpers.WaitGroupCount, checkSumsTracker *progress.Tracker) {
	if count != nil {
		defer count.Done()
	}
	file, err := os.Open(path.Join(a.Settings.TmpDir, build.FileSpec.Name+"."+build.FileSpec.HttpInfo.FileType))
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		checkSumsTracker.MarkAsErrored()
	}
	if build.FileSpec.HttpInfo.HashValue == hex.EncodeToString(hash.Sum(nil)) {
		//checkSumsTracker.SetValue(int64(i))
		checkSumsTracker.Increment(1)
		build.FileSpec.ValidCheckSum = true
	} else {
		checkSumsTracker.UpdateMessage(text.FgRed.Sprintf("Invalid check sum of %s", build.FileSpec.Name))
		checkSumsTracker.MarkAsErrored()
		build.FileSpec.Error = errors.New("can't validate checksum").Error()
		build.FileSpec.Status = structs.Errored
	}
}

func (a *Agent) unpackBuildFile(build *structs.Build, count *helpers.WaitGroupCount, progressTrack progress.Writer) {
	if count != nil {
		defer count.Done()
	}
	unpackTracker := helpers.NewTracker(0,
		nil,
		nil,
		fmt.Sprintf("Unpack %s", build.FileSpec.Name))
	progressTrack.AppendTracker(unpackTracker)
	switch build.FileSpec.HttpInfo.FileType {
	case "zip":
		dst := path.Join(a.Settings.TmpDir, build.FileSpec.Name)
		archive, err := zip.OpenReader(path.Join(a.Settings.TmpDir,
			build.FileSpec.Name+"."+build.FileSpec.HttpInfo.FileType))
		unpackTracker.UpdateTotal(int64(len(helpers.Filter(archive.File, func(file *zip.File) bool {
			return !file.FileInfo().IsDir()
		}))))
		if err != nil {
			unpackTracker.MarkAsErrored()
		}
		defer archive.Close()

		for _, f := range archive.File {
			filePath := filepath.Join(dst, f.Name)

			if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
				unpackTracker.UpdateMessage(text.FgRed.Sprint("Invalid file path in archive"))
				unpackTracker.MarkAsErrored()
				return
			}
			if f.FileInfo().IsDir() {
				os.MkdirAll(filePath, os.ModePerm)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				unpackTracker.MarkAsErrored()
			}
			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			fileInArchive, err := f.Open()
			_, err = io.Copy(dstFile, fileInArchive)
			if err != nil {
				build.FileSpec.Status = structs.Errored
				build.FileSpec.Error = err.Error()
				unpackTracker.MarkAsErrored()
			} else {
				unpackTracker.Increment(1)
			}
			dstFile.Close()
			fileInArchive.Close()
		}
		build.FileSpec.Status = structs.Unpacked
		unpackTracker.MarkAsDone()
		unpackTracker.UpdateMessage(text.FgGreen.Sprintf("Unpack %s", build.FileSpec.Name))
	default:
		build.FileSpec.Error = errors.New(fmt.Sprintf("unknown algorithm to unpack build: %s",
			build.FileSpec.HttpInfo.FileType)).Error()
		build.FileSpec.Status = structs.Errored
		log.Log.Fatal().Msg("Unknown algorithm to unpack build")
	}
}

func (a *Agent) downloadPackage(targetPackage *structs.Package) {
	// INIT PROGRESS BAR {{
	progressTrack := *helpers.NewProgressBar(len(targetPackage.PackageItems)*2+1, 1)
	progressTrack.SetMessageWidth(50)
	//progressTrack.SetPinnedMessages("Download Files")
	go progressTrack.Render()
	// }}

	wg := helpers.WaitGroupCount{}
	wg.Add(len(targetPackage.PackageItems))

	// START DOWNLOAD BUILDS
	for _, packageItem := range targetPackage.PackageItems {
		a.downloadSoftware(packageItem.Software, &wg, progressTrack)
	}

	time.Sleep(2 * time.Millisecond)
	wg.Wait()

	// VALIDATE HASH {{
	checkSumsTracker := helpers.NewTracker(len(targetPackage.PackageItems),
		nil,
		nil,
		fmt.Sprintf("Validate chek sums"))
	progressTrack.AppendTracker(checkSumsTracker)
	wg.Add(len(targetPackage.PackageItems))
	for _, packageItem := range targetPackage.PackageItems {
		go a.validateBuildCheckSum(packageItem.Software.Build, &wg, checkSumsTracker)
	}
	time.Sleep(5 * time.Millisecond)
	wg.Wait()
	checkSumsTracker.MarkAsDone()
	checkSumsTracker.UpdateMessage(text.FgGreen.Sprint("Validate check sums"))
	// }}

	// UNPACK BUILDS {{
	//progressTrack.SetPinnedMessages("Unpack builds")
	wg.Add(len(targetPackage.PackageItems))
	for _, packageItems := range targetPackage.PackageItems {
		go a.unpackBuildFile(packageItems.Software.Build, &wg, progressTrack)
	}
	time.Sleep(2 * time.Millisecond)
	wg.Wait()
	//progressTrack.SetPinnedMessages("Done")
	// }}

	time.Sleep(2 * time.Millisecond)
	progressTrack.Stop()
}

func (a *Agent) configureSoftware(softwareId int) {
	config := a.ApiClient.GetSoftwareConfig(softwareId)
	if config != nil {
		err := os.WriteFile(config.Path, []byte(config.RawData), 0666)
		if err != nil {
			log.Log.Error().Err(err).Msgf("Failed to write software config file")
			return
		} else {
			log.Log.Info().Msgf("Wrote software config file: %s", config.Path)
			a.FilesIterInstalledPackageItem(func(info *SavedInfo, packageItem *structs.PackageItem) {
				if packageItem.Software.ID == softwareId {
					if packageItem.Software.PackageInfo.ServiceInfo != nil {
						packageItem.Software.PackageInfo.ServiceInfo.Restart()
					}
					if packageItem.Software.PackageInfo.AutorunInfo != nil {
						packageItem.Software.PackageInfo.AutorunInfo.Restart()
					}
				}
			})
		}
	} else {
		log.Log.Info().Msgf("Config to software %d not found", softwareId)
		return
	}
}

func (a *Agent) createNotification(ntype string, context ...map[string]any) *structs.RestNotifyPost {
	notification := &structs.RestNotifyPost{
		Type:    ntype,
		Context: MergeMaps(context...),
	}
	if a.Tmp != nil {
		notification.PackageId = a.Tmp.Package.ID
		notification.UnitID = a.Tmp.Unit.ID
	}
	if a.CommandId != nil {
		notification.CmdTriggerId = a.CommandId
	}
	return notification
}

func (a *Agent) getApplicationUpdate(params *structs.ApplicationUpdateParams) (*bool, *string, error) {
	var extKey = ""
	var existVersion *version.Version
	var existPatchNum int
	a.FilesIterInstalledPackageItem(func(info *SavedInfo, packageItem *structs.PackageItem) {
		if packageItem.Software.ExternalKey != nil {
			if strings.Contains(*packageItem.Software.ExternalKey, params.ExternalKey) {
				existVersion, _ = version.NewVersion(packageItem.Software.Version)
				existPatchNum = packageItem.Software.Patch
				extKey = *packageItem.Software.ExternalKey
			}
		}
	})
	if extKey == "" {
		return helpers.FalsePtr(), nil, errors.New("ext key not found")
	}
	if existVersion.LessThan(params.Version) {
		return helpers.FalsePtr(), nil, errors.New("can't find new version")
	}
	if existVersion.Equal(params.Version) && existPatchNum <= params.PatchNumber {
		return helpers.FalsePtr(), nil, errors.New("can't find new version")
	}
	var targetFile = ""

	err := filepath.Walk(path.Join(a.Settings.AppFolder, extKey), func(path string, info fs.FileInfo, err error) error {
		if info.Name() == params.FileName && !info.IsDir() {
			targetFile = path
		}
		return err
	})
	if err != nil {
		return helpers.FalsePtr(), nil, err
	}
	if targetFile != "" {
		return helpers.TruePtr(), &targetFile, nil
	} else {
		return helpers.FalsePtr(), nil, errors.New("can't find target file")
	}
}

// PROCESS

func (a *Agent) RegProcess() {
	sentExt := SupportedPkgExt[RunPkgManager]
	sentPkgName := strings.Split(sentExt, ".")[1]
	ip := helpers.GetLocalIP()
	var ipStr string
	switch ip {
	case nil:
		ipStr = "0.0.0.0"
	default:
		ipStr = ip.String()
	}
	_, offset := time.Now().Zone()
	isReg := a.ApiClient.Reg(&structs.RestRegPost{LocalAddress: ipStr,
		AgentSecret:     a.Settings.SECRET,
		Version:         PcaVersion,
		PkgSystem:       sentPkgName,
		System:          OsVersion(),
		LocalTimeOffset: offset})
	if isReg {
		log.Log.Info().Msgf("Agent registration complete")
	} else {
		log.Log.Error().Msgf("Agent registration fails")
	}
}

func (a *Agent) ConfigureProcess(softwareId *int) {
	a.FilesIterInstalledPackageItem(func(info *SavedInfo, packageItem *structs.PackageItem) {
		if packageItem.Software.ID == *softwareId {
			a.configureSoftware(packageItem.Software.ID)
		}
		return
	})
}

func (a *Agent) installSoftware(software *structs.Software, inst *install.Installer) error {
	onError := func(build *structs.Build, err error) {
		build.FileSpec.Error = err.Error()
		build.FileSpec.Status = structs.Errored
		log.Log.Error().Err(err).Msgf("Failed to install packages")
		a.ApiClient.Notify(a.createNotification("fails",
			map[string]any{"error": err.Error()}))
		a.FilesSerializeInstallation()
	}

	build := software.Build
	var installError error
	if software.Kind == "application" {
		key := software.ExternalKey
		if key != nil {
			software.PackageInfo, installError = inst.InstallPackage(software.Kind,
				path.Join(a.Settings.TmpDir, software.Build.FileSpec.Name),
				path.Join(a.Settings.AppFolder, *key))
		}
	} else {
		filepath.Walk(path.Join(a.Settings.TmpDir, software.Build.FileSpec.Name),
			func(filePath string, info fs.FileInfo, err error) error {
				if !info.IsDir() && strings.Contains(info.Name(), SupportedPkgExt[RunPkgManager]) {
					software.PackageInfo, installError = inst.InstallPackage(software.Kind, filePath)
				}
				return err
			},
		)
	}

	if installError != nil {
		onError(build, installError)
		inst.RollbackAll()
		return installError
	}
	if software.Kind != "application" {
		a.configureSoftware(software.ID)
	}
	software.Build.FileSpec.Status = structs.Installed
	return nil
}

func (a *Agent) InstallPackage(tPackage *structs.Package) error {
	a.ApiClient.Notify(a.createNotification("download"))
	a.downloadPackage(tPackage)

	sort.Slice(tPackage.PackageItems, func(i, j int) bool {
		return tPackage.PackageItems[i].InstallOrder < tPackage.PackageItems[j].InstallOrder
	})

	inst := install.NewInstaller(RunPkgManager, RunFlags)
	for _, packageItem := range tPackage.PackageItems {
		//build := packageItem.Software.Build
		installError := a.installSoftware(packageItem.Software, inst)
		if installError != nil {
			return installError
		}
	}
	a.ApiClient.Notify(a.createNotification("installed"))
	a.FilesSerializeInstallation()
	return nil
}

func (a *Agent) patchSoftware(software *structs.Software) error {
	// INIT PROGRESS BAR {{
	progressTrack := *helpers.NewProgressBar(3, 1)
	progressTrack.SetMessageWidth(50)
	go progressTrack.Render()
	// }}

	// START DOWNLOAD BUILDS
	a.downloadSoftware(software, nil, progressTrack)

	// VALIDATE HASH {{
	checkSumsTracker := helpers.NewTracker(1,
		nil,
		nil,
		fmt.Sprintf("Validate chek sums"))
	progressTrack.AppendTracker(checkSumsTracker)
	a.validateBuildCheckSum(software.Build, nil, checkSumsTracker)
	checkSumsTracker.MarkAsDone()
	checkSumsTracker.UpdateMessage(text.FgGreen.Sprint("Validate check sums"))
	// }}

	// UNPACK BUILD
	a.ApiClient.Notify(a.createNotification("download"))
	a.unpackBuildFile(software.Build, nil, progressTrack)

	time.Sleep(2 * time.Millisecond)
	progressTrack.Stop()

	// INSTALL SOFTWARE

	inst := install.NewInstaller(RunPkgManager, RunFlags)
	//build := packageItem.Software.Build
	installError := a.installSoftware(software, inst)
	if installError != nil {
		return installError
	}
	a.ApiClient.Notify(a.createNotification("installed"))
	a.FilesSerializeInstallation()
	return nil
}

func (a *Agent) updatePackage(info *SavedInfo) {
	log.Log.Info().Msgf("Check packages for %s", info.Package.Name)
	updates := a.ApiClient.GetSoftwareUpdates(info.Unit.ID)
	upd := helpers.Find(updates, func(pac *structs.Package) bool {
		if *pac.PrevPackageID == info.Package.ID {
			for _, packItem := range pac.PackageItems {
				if !packItem.Enable {
					return false
				}
			}
			return true
		}
		return false
	})
	if upd != nil {
		log.Log.Info().Msgf("Find update for %s", info.Package.Name)
		t := helpers.ConstructTable(&table.Row{"Package", "Software", "Update"})
		for _, installedPi := range info.Package.PackageItems {
			for _, newPi := range (*upd).PackageItems {
				if installedPi.Software.Name == newPi.Software.Name {
					t.AppendRow(table.Row{info.Package.Name,
						installedPi.Software.Name,
						VersionStringColor(installedPi.Software, newPi.Software)})
				}
			}
		}
		t.Render()
		agree := AskConfirm(ForceCmd)
		if agree {
			info.Package = *upd
			a.Tmp = info
			err := a.InstallPackage(*upd)
			if err != nil {
				log.Log.Error().Err(err).Msgf("Update %s FAILED", info.Package.Name)
				return
			}
		}
	} else {
		log.Log.Info().Msgf("No updates for %s", info.Package.Name)
	}
}

func (a *Agent) UpdateProcess(packageId *int, innerIndex *int) {
	var updatePackage *SavedInfo = nil
	if *packageId != -1 || *innerIndex != -1 {
		if *packageId != -1 {
			a.FilesIterInstalled(func(info *SavedInfo) {
				if info.Package.ID == *packageId {
					updatePackage = info
				}
			})
		}
		if *innerIndex != -1 {
			a.FilesIterInstalled(func(info *SavedInfo) {
				if info.Package.InnerIndex == *packageId {
					updatePackage = info
				}
			})
		}

		if updatePackage == nil {
			log.Log.Info().Msgf("Package %d not found, please specify correct idx", *packageId)
			return
		}
		a.updatePackage(updatePackage)
	} else {
		a.FilesIterInstalled(func(info *SavedInfo) {
			a.updatePackage(info)
		})
	}
}

func (a *Agent) PatchProcess(softwareId *int) {
	a.FilesIterInstalled(func(info *SavedInfo) {
		patchSoftware := func(installedSoftware *structs.Software) {
			software := a.ApiClient.GetSoftwareLatestPatch(installedSoftware.ID)

			if software == nil {
				log.Log.Info().Msgf("Remote software (Package name: %s, SoftwareId: %d) not found",
					info.Package.Name, installedSoftware.ID)
				return
			}

			if software.Patch <= installedSoftware.Patch {
				log.Log.Info().Msgf("Software %s already has latest patch", installedSoftware.Name)
				return
			}

			log.Log.Info().Msgf("Find new patch for %s", info.Package.Name)
			t := helpers.ConstructTable(&table.Row{"Software", "Update"})
			t.AppendRow(table.Row{software.Name, VersionStringColor(installedSoftware, software)})
			t.Render()
			agree := AskConfirm(ForceCmd)
			if agree {
				for _, pi := range info.Package.PackageItems {
					if pi.Software.ID == installedSoftware.ID {
						pi.Software = software
					}
				}
				a.Tmp = info
				err := a.patchSoftware(software)
				if err != nil {
					log.Log.Error().Err(err).Msgf("Patch %s FAILED", info.Package.Name)
					return
				}
			}
		}
		if *softwareId != 0 {
			var founded = false
			for _, pi := range info.Package.PackageItems {
				if *softwareId == pi.Software.ID {
					log.Log.Info().Msgf("Check packages for %s", info.Package.Name)
					patchSoftware(pi.Software)
					founded = true
					break
				}
			}
			if !founded {
				log.Log.Info().Msgf("Not found software with id: %d in installed packages, please specify correct idx", *softwareId)
			}
		} else {
			log.Log.Info().Msgf("Check packages for %s", info.Package.Name)
			for _, pi := range info.Package.PackageItems {
				patchSoftware(pi.Software)
			}
		}
	})
}

func (a *Agent) InstallProcess(clientId *int, productId *int, packageId *int) {
	clients := a.ApiClient.GetAllClients()
	targetClient := AskTargetClient(clientId, clients)
	if targetClient == nil {
		log.Log.Error().Msg("Client not fetched")
		return
	}
	products := a.ApiClient.GetAllProductsInClient(targetClient.ID)
	targetProduct := AskTargetProduct(productId, products)
	if targetProduct == nil {
		log.Log.Error().Msg("Product not fetched")
		return
	}
	sentLimit := map[bool]int{true: 5, false: 1}[*installVerbose]
	units := a.ApiClient.GetAllUnitsInProduct(targetProduct.ID, sentLimit)
	targetUnit, targetPackage := AskTargetPackage(targetProduct, units, packageId)
	if targetPackage == nil {
		log.Log.Error().Msg("Package not fetched")
		return
	}
	a.FilesAddInstallation(targetClient, targetProduct, targetPackage, targetUnit)
	a.InstallPackage(targetPackage)
}

func (a *Agent) RemovePackages(packageIds ...int) {
	for _, id := range packageIds {
		installedPackage := a.FilesGetInfoByPackageID(id)
		if installedPackage == nil {
			log.Log.Warn().Msgf("package with id %d not found, skip", id)
			continue
		}
		log.Log.Info().Msgf("Delete package %s", installedPackage.Name)
		confirm := AskConfirm(ForceCmd)
		if confirm {
			inst := install.NewInstaller(RunPkgManager, RunFlags)
			for _, pi := range installedPackage.PackageItems {
				if pi.Software.Build.FileSpec.Status != structs.Installed {
					log.Log.Info().Msgf("Package %s was not installed, skip", pi.Software.Name)
					a.FilesForgetPackage(id)
					continue
				}
				if pi.Software.Kind == "application" {
					inst.AddInstalledPackage(pi.Software.Kind, path.Join(a.Settings.AppFolder, *pi.Software.ExternalKey))
				} else {
					inst.AddInstalledPackage(pi.Software.Kind, pi.Software.PackageInfo.ControlInfo.PackageName)
				}
			}

			err := inst.RollbackAll()
			if err != nil {
				log.Log.Error().Err(err).Msgf("can't remove software in package %s", installedPackage.Name)
			} else {
				a.FilesForgetPackage(id)
			}
		}
	}
	a.FilesSerializeInstallation()
}

func (a *Agent) RemoteShellProcess() error {
	return nil
	//ws, wsError := a.RestClient.OpenWebsocket()
	//if wsError != nil {
	//	log.Log.Error().Err(wsError).Msg("Unable to start websocket session ")
	//	return wsError
	//}
	//return a.remoteShellWrite(ws)

	//reader := bufio.NewReader(os.Stdin)
	//for b, err := reader.ReadByte(); err == nil; b, err = reader.ReadByte() {
	//	stdin.Write([]byte{b})
	//}
}
