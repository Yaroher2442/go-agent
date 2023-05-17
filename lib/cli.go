package lib

import (
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	"main/lib/helpers"
	"main/lib/log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var (
	Commander = kingpin.New("lib", "Product control agent")

	VersionCmd = Commander.Command("version", "Show version").Alias("v")
	ForceCmd   = Commander.Flag("force", "Skip confirm").Short('f').Bool()

	innerIndexFlag = Commander.Flag("inner-index", "Inner index").Short('i').Default("-1").Int()

	reg = Commander.Command("reg", "Register Agent in Pc system")

	installCmd       = Commander.Command("install", "InstallProcess software")
	installClientId  = installCmd.Flag("client", "Client id to installation").Short('c').Int()
	installPrId      = installCmd.Flag("product", "Product id to installation").Short('r').Int()
	installPackageId = installCmd.Flag("package", "Product id to installation").Short('p').Int()
	installVerbose   = installCmd.Flag("verbose", "Show latest 5 uints to chose").Short('v').Bool()
	remove           = Commander.Command("remove", "Remove installed package")
	removePackageIds = remove.Flag("package", "Package to remove").Short('p').Int64List()
	removeAll        = remove.Flag("all", "To remove all installations").Bool()
	updateCmd        = Commander.Command("update", "Check and install update")
	updatePackageId  = updateCmd.Flag("package", "Update one concrete package").Short('p').Default("-1").Int()
	patch            = Commander.Command("patch", "Check and install patches")
	patchSoftwareId  = patch.Flag("software", "Patch one concrete software").Short('s').Default("0").Int()

	selfUpdate = updateCmd.Flag("self", "Check and install self agent update").Short('s').Bool()

	listCmd        = Commander.Command("list", "List installed products")
	listCmdVerbose = listCmd.Flag("verbose", "List installed products").Short('v').Bool()

	reconfigure = Commander.Command("reconf", "Open editor with pca config file")
	shell       = Commander.Command("shell", "Open remote shell")

	soft           = Commander.Command("soft", "Operation on installed software")
	softSoftwareId = soft.Flag("software", "Software id to operate on").Short('s').Default("-1").Int()
	softConfig     = soft.Command("config", "Check remote software config")

	service        = Commander.Command("service", "Manipulate service")
	removeService  = service.Command("remove", "Remove systemctl pca service (stop and delete service info)")
	installService = service.Command("install", "Install systemctl pca service (only create service)")
	startService   = service.Command("start", "Start systemctl pca service (start existed service)")
	stopService    = service.Command("stop", "Stop systemctl pca service (stop existed service)")
	restartService = service.Command("restart", "Restart restart pca service (restart existed service)")
	statusService  = service.Command("status", "Status of systemctl pca service")
	serveService   = service.Command("serve", "Serve pca service (system usage only, start service in current process)")
)

func SelectCommand(command []string, agent *Agent) error {

	args := kingpin.MustParse(Commander.Parse(command))
	log.Log.Debug().Msgf("catch cmd %s", args)
	handleInterrupt := make(chan os.Signal, 1)
	signal.Notify(handleInterrupt, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		<-handleInterrupt
		agent.RemoteUnlock()
		os.Exit(0)
	}()

	switch args {
	case VersionCmd.FullCommand():
		log.Log.Info().Msgf("version is %s", PcaVersion)
	// info
	case reg.FullCommand():
		HandleRoot()
		agent.WithRemoteLock(func() {
			agent.RegProcess()
		})
	case listCmd.FullCommand():
		agent.WithRemoteLock(func() {
			agent.DisplayInstalled()
		})
	case reconfigure.FullCommand():
		HandleRoot()
		oldConfig := agent.Settings
		editor := FindInstalledEditor()
		captureErr, err := helpers.ShellOutCaptureErr(fmt.Sprintf("%s %s", editor, ConfigPath))
		if err != nil || captureErr != "" {
			SafeWriteJsonFile(oldConfig, nil, ConfigPath, 0666)
			log.Log.Warn().Err(err).Msgf("err: %s, config restored", captureErr)
		}
		if agent.RpcClient != nil {
			var status int
			err := agent.RpcClient.Call("Agent.RpcReconfigure", &status, &status)
			if err != nil {
				agent.RemoteUnlock()
				log.Log.Fatal().Err(err).Msg("RpcReconfigure failed")
			}
		}
		log.Log.Info().Msg("Service was reconfigured")
	// software manipulate
	case installCmd.FullCommand():
		HandleRoot()
		if installPackageId != nil && (installClientId == nil || installPrId == nil) {
			log.Log.Error().Msg("Can't pass packageId (--package/-p) " +
				"without clientId (--client/-c), and productId (--product/-r)")
		}
		if installPrId != nil && installClientId == nil {
			log.Log.Error().Msg("Can't pass productId (--product/-r) without clientId (--client/-c)")
		}
		agent.WithRemoteLock(func() {
			agent.InstallProcess(installClientId, installPrId, installPackageId)
		})
	case remove.FullCommand():
		HandleRoot()
		agent.WithRemoteLock(func() {
			removeList := make([]int, 0)
			if *removeAll {
				agent.FilesIterInstalled(func(info *SavedInfo) {
					removeList = append(removeList, info.Package.ID)
				})
				log.Log.Warn().Msg("All packages will be removed!")
			} else {
				if len(*removePackageIds) == 0 {
					agent.DisplayInstalled()
					AskInput("number of package", func(input string) bool {
						num, err := strconv.Atoi(input)
						if err != nil {
							log.Log.Info().Msg("Please pass int value")
							return false
						}
						for _, info := range agent.Installed {
							if info.Package.InnerIndex == num {
								removeList = append(removeList, info.Package.ID)
								return true
							}
						}
						log.Log.Info().Msg("This package not found, please retry")
						return false
					})
				} else {
					agent.FilesIterInstalled(func(info *SavedInfo) {
						if helpers.Contains(*removePackageIds, int64(info.Package.ID)) {
							removeList = append(removeList, info.Package.ID)
						}
					})
				}
			}
			if len(removeList) == 0 {
				log.Log.Info().Msg("Nothing to remove")
			}
			agent.RemovePackages(removeList...)
		})
	case updateCmd.FullCommand():
		HandleRoot()
		if *selfUpdate {
			updateWrapper := NewAgentUpdateWrap(agent)
			updateWrapper.SelfUpdateProcess()
		} else {
			agent.WithRemoteLock(func() {
				agent.UpdateProcess(updatePackageId, innerIndexFlag)
			})
		}
	case patch.FullCommand():
		HandleRoot()
		agent.WithRemoteLock(func() {
			agent.PatchProcess(patchSoftwareId)
		})
	case softConfig.FullCommand():
		HandleRoot()
		agent.WithRemoteLock(func() {
			if *softSoftwareId == -1 {
				agent.DisplayInstalled()
				log.Log.Warn().Msg("Please pass software id")
			}
			agent.ConfigureProcess(softSoftwareId)
		})
	case shell.FullCommand():
		//if err := test(); err != nil {
		//	log.Log.Fatal().Err(err)
		//}
		agent.WithRemoteLock(func() {
			err := agent.RemoteShellProcess()
			if err != nil {
				log.Log.Fatal().Err(err).Msg("Failed to start remote shell")
			}
		})
	}
	if strings.Contains(args, service.FullCommand()) {
		agentService := NewAgentServiceWrap(agent)
		var err error
		var status = ""
		switch args {
		case installService.FullCommand():
			HandleRoot()
			status, err = agentService.Install([]string{"service", "serve"}...)
		case removeService.FullCommand():
			HandleRoot()
			status, err = agentService.Remove()
		case statusService.FullCommand():
			HandleRoot()
			status, err = agentService.Status()
		case startService.FullCommand():
			HandleRoot()
			status, err = agentService.Start()
		case stopService.FullCommand():
			HandleRoot()
			status, err = agentService.Stop()
		case restartService.FullCommand():
			HandleRoot()
			status, err = agentService.Stop()
			status, err = agentService.Start()
		case serveService.FullCommand():
			HandleRoot()
			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)
			signal.Reset()
			agentService.OnServiceStart()
			killSignal := <-interrupt
			log.Log.Info().Msgf("Got signal:", killSignal)
			agentService.OnServiceStop()
		}
		if err != nil {
			log.Log.Error().Err(err).Msg(status)
		}
	}

	return nil
}
