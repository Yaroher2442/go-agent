package structs

import (
	"github.com/hashicorp/go-version"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/takama/daemon"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APP FETCH PARAMS -----------------------------------------

type ApplicationUpdateParams struct {
	ExternalKey string
	FileName    string
	Version     *version.Version
	PatchNumber int
}

func ApplicationUpdateParamsFromReq(request *http.Request) (*ApplicationUpdateParams, error) {
	QPatchNumber := request.URL.Query().Get("patch")
	patchNum, err := strconv.Atoi(QPatchNumber)
	versionName := request.URL.Query().Get("version")
	newVersion, err := version.NewVersion(versionName)
	if err != nil {
		return nil, err
	}
	return &ApplicationUpdateParams{
		ExternalKey: request.URL.Query().Get("ext_key"),
		FileName:    request.URL.Query().Get("file_name"),
		Version:     newVersion,
		PatchNumber: patchNum,
	}, nil

}

// ---------------------BUILD--------------------------------------

type BuildStatus string

const (
	Created    BuildStatus = "Created"
	Downloaded             = "Downloaded"
	Installed              = "Installed"
	Unpacked               = "Unpacked"
	Errored                = "Errored"
)

type BuildInfo struct {
	Status        BuildStatus   `json:"status" groups:"local"`
	ValidCheckSum bool          `json:"valid_check_sum" groups:"local"`
	LoadedBytes   int64         `json:"loaded_bytes" groups:"local"`
	HttpInfo      *HttpFileSpec `json:"http_info" groups:"local"`
	Name          string        `json:"name" groups:"local"`
	Error         string        `json:"error" groups:"local"`
}

func NewBuildInfo(name string, fileSpec *HttpFileSpec) *BuildInfo {
	name = strings.ReplaceAll(name, " ", "_")
	return &BuildInfo{
		Status:      Created,
		LoadedBytes: 0,
		HttpInfo:    fileSpec,
		Name:        name,
		Error:       "",
	}
}

// ---------------------INFO--------------------------------------

type Client struct {
	ID          int    `json:"id" groups:"local"`
	Description string `json:"description" groups:"local"`
	Name        string `json:"name" groups:"local"`
}

func (client *Client) Row() *table.Row {
	return &table.Row{client.Name, client.Description}
}

func (client *Client) Header() *table.Row {
	return &table.Row{"#", "Name", "Description"}
}

type Product struct {
	ID          int    `json:"id" groups:"local"`
	Name        string `json:"name" groups:"local"`
	Description string `json:"description" groups:"local"`
}

func (pr *Product) Row() *table.Row {
	return &table.Row{pr.Name, pr.Description}
}

func (pr *Product) Header() *table.Row {
	return &table.Row{"#", "Name", "Description"}
}

// ----------------------USAGE-------------------------------------

type Build struct {
	ID       int        `json:"id" groups:"local"`
	Name     string     `json:"name" groups:"local"`
	HashSum  string     `json:"hashsum" groups:"local"`
	FileSpec *BuildInfo `json:"file_spec" groups:"local"` // system usage
}

type Software struct {
	ID          int     `json:"id" groups:"local"`
	Name        string  `json:"name" groups:"local"`
	Kind        string  `json:"kind" groups:"local"`
	Branch      string  `json:"branch" groups:"local"`
	Version     string  `json:"version" groups:"local"`
	ExternalKey *string `json:"external_key" groups:"local"`
	Patch       int     `json:"patch" groups:"local"`
	Build       *Build  `json:"build" groups:"local"`

	PackageInfo *Installation `json:"package_info" groups:"local"` // system usage
	FullName    *string       `json:"full_name" groups:"local"`
	Error       string        `json:"error" groups:"local"`

	GitlabID    int       `json:"gitlab_id"`
	Description string    `json:"description"`
	Changelog   *string   `json:"changelog"`
	URL         string    `json:"url"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (soft *Software) GetSysPackageName() string {
	if soft.PackageInfo.ControlInfo == nil {
		return ""
	}
	return soft.PackageInfo.ControlInfo.PackageName

}

func (soft *Software) GetServiceInfo() (string, string) {
	if soft.PackageInfo.ServiceInfo == nil {
		return "", ""
	}
	serviceDaemon, _ := daemon.New(
		strings.Split(soft.PackageInfo.ServiceInfo.Name, ".")[0],
		"",
		daemon.SystemDaemon,
		[]string{}...,
	)
	status, err := serviceDaemon.Status()
	if err != nil {
		return "", "Unable to get info"
	}
	if strings.Contains(status, "stopped") {
		status = text.FgRed.Sprint(status)
	}
	return soft.PackageInfo.ServiceInfo.Name, status
}

func (soft *Software) String() string {
	return soft.Branch + "_" + soft.Version + "+" + strconv.Itoa(soft.Patch)
}

func (soft *Software) StringWithName() string {
	return soft.Name + "_" + soft.Branch + "_" + soft.Version + "+" + strconv.Itoa(soft.Patch)
}

type PackageItem struct {
	InstallOrder int       `json:"install_order" groups:"local"`
	PackageID    int       `json:"package_id" groups:"local"`
	Software     *Software `json:"software" groups:"local"`

	Enable bool `json:"enable" groups:"api"`
}

type Package struct {
	ID           int            `json:"id" groups:"local"`
	Name         string         `json:"name" groups:"local"`
	Description  string         `json:"description" groups:"local"`
	PackageItems []*PackageItem `json:"packageitems" groups:"local"`

	InnerIndex int `json:"inner_index" groups:"local"` // system usage

	PrevPackageID *int `json:"prev_package_id,omitempty"`
}

type Unit struct {
	ID        int        `json:"id" groups:"local"`
	ProductID int        `json:"product_id" groups:"local"`
	Status    string     `json:"status" groups:"local"`
	Packages  []*Package `json:"packages"`
}
