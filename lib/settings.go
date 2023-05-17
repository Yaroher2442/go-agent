package lib

import (
	"errors"
	"github.com/google/uuid"
	"github.com/liip/sheriff"
	"main/lib/helpers"
	"main/lib/log"
	"main/lib/structs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

const PcaVersion = "1.2.3" // do not change! Changes automatically in CI
const ServiceDescription = "abt-tech packages agent"
const ServiceName = "pca"
const CommandsTimeout = 60

var (
	DEBUG                = helpers.FalsePtr()
	ServiceDependencies  = []string{}
	ConfigPath           = ""
	RrcDns               = ":38845"
	HttpDns              = ":38844"
	SupportedPkgManagers = []string{"apt"}
	SupportedPkgExt      = map[string]string{"apt": ".deb"}

	RunPkgManager = ""
	RunFlags      = ""

	DefaultPkgFlags = map[string]string{"apt": "-oAcquire::AllowUnsizedPackages=1 -oAcquire::http::Pipeline-Depth=0 -y -qq --allow-downgrades"}

	JsonLocalOpts = &sheriff.Options{
		Groups: []string{"local"},
	}
)

func HandleRoot() {
	if !helpers.Must(helpers.IsRoot()) {
		log.Log.Fatal().Msg("To use Abt-Agent please get sudo privileges")
	}
}

func FindPackageSystem() {
	for _, name := range SupportedPkgManagers {
		_, err := exec.LookPath(name)
		if err == nil {
			RunPkgManager = name
		}
	}
	if RunPkgManager == "" {
		log.Log.Fatal().Err(errors.New("run package manager not determined"))
	}

}

func ConfigurePaths() {
	homePath := "/opt/abt/pca"
	ConfigPath = filepath.Join(homePath, "config.json")
	systemPath := filepath.Join(homePath, "system")
	logsPath := filepath.Join(systemPath, "logs")
	AppFolder := filepath.Join(systemPath, "apps")
	rootPath := path.Dir(ConfigPath)
	installPath := path.Join(rootPath, "install.d")
	tmpPath := path.Join(systemPath, "tmp")

	helpers.FileExists(rootPath)
	for _, _path := range []string{rootPath,
		installPath,
		AppFolder,
		tmpPath,
		logsPath} {
		if !helpers.FileExists(_path) {
			HandleRoot()
		}
		err := os.MkdirAll(_path, os.ModeDir|os.ModePerm)
		if err != nil {
			log.Log.Fatal().Err(err).Msg("Can't create pca dirs, try admin permissions")
		}
	}

	logFile := path.Join(logsPath, "pca.log")
	if !helpers.FileExists(logFile) {
		err := os.WriteFile(logFile, make([]byte, 0), 0666)
		if err != nil {
			log.Log.Fatal().Err(err).Msg("Can't create log file, try admin permissions")
		}
	}
}

func DefaultSettings() *Settings {
	return &Settings{
		DEBUG:     false,
		SECRET:    uuid.NewString(),
		HttpPort:  HttpDns,
		RpcPort:   RrcDns,
		InfoDir:   path.Join(path.Dir(ConfigPath), "install.d"),
		SystemDir: path.Join(path.Dir(ConfigPath), "system"),
		TmpDir:    path.Join(path.Dir(ConfigPath), "system", "tmp"),
		AppFolder: path.Join(path.Dir(ConfigPath), "system", "apps"),
		LogDir:    path.Join(path.Dir(ConfigPath), "system", "logs"),
		PkgFlags:  DefaultPkgFlags,
		NetInfo: &NetSettings{
			Protocol:    "https",
			ControlIp:   "release.a-7.tech",
			ControlPort: "",
			AsProxy:     false,
		},
		RemoteCommandsEnabled: true,
		CommandsTimeout:       CommandsTimeout,
	}
}

type NetSettings struct {
	Protocol    string `json:"protocol"`
	ControlIp   string `json:"control_ip"`
	ControlPort string `json:"control_port"`
	AsProxy     bool   `json:"as_proxy"`
}

type Settings struct {
	DEBUG                 bool              `json:"debug"`
	SECRET                string            `json:"secret"`
	HttpPort              string            `json:"http_port"`
	RpcPort               string            `json:"rpc_port"`
	InfoDir               string            `json:"info_dir"`
	SystemDir             string            `json:"system_dir"`
	AppFolder             string            `json:"app_folder"`
	TmpDir                string            `json:"tmp_dir"`
	LogDir                string            `json:"log_dir"`
	PkgFlags              map[string]string `json:"pkg_flags"`
	NetInfo               *NetSettings      `json:"net_info"`
	RemoteCommandsEnabled bool              `json:"remote_commands_enabled"`
	CommandsTimeout       int               `json:"commands_timeout"`
}

func LoadSettings() *Settings {
	settings := DefaultSettings()
	if helpers.FileExists(ConfigPath) {
		SafeReadJsonFile(ConfigPath, settings)
	} else {
		os.WriteFile(ConfigPath, make([]byte, 0), 0666)
		SafeWriteJsonFile(settings, nil, ConfigPath, 0666)
	}
	softLogFile := filepath.Join(settings.LogDir, "soft.log.json")
	if !helpers.FileExists(softLogFile) {
		os.WriteFile(softLogFile, make([]byte, 0), 0666)
		SafeWriteJsonFile(&LogBufferFile{Logs: make([]*structs.RestLogPost, 0)}, nil, softLogFile, 0666)
	}
	RunFlags = settings.PkgFlags[RunPkgManager]
	*DEBUG = settings.DEBUG
	log.OverrideLogger(*DEBUG, settings.LogDir)
	return settings
}
