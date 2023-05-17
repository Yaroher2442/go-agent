package structs

// BASES -------------------------------------------------

type ApiInconsistencyContextErr struct {
	Type     string `json:"type"`
	Code     string `json:"code"`
	Class    int    `json:"class"`
	Subclass int    `json:"subclass"`
	Comment  string `json:"comment"`
}

type ApiInconsistencyContext struct {
	Description string                     `json:"description"`
	Status      int                        `json:"status"`
	ErrorCtx    ApiInconsistencyContextErr `json:"error"`
}

type ApiAccessWithTotal[T interface{}] struct {
	Items []*T `json:"items"`
	Total int  `json:"total"`
}

// POST DTO -------------------------------------------------

type RestRegPost struct {
	AgentSecret     string         `json:"agent_secret"`
	LocalAddress    string         `json:"local_address"`
	System          string         `json:"system"`
	PkgSystem       string         `json:"pkg_system"`
	ProxyInfo       map[int]string `json:"proxy_info"`
	Version         string         `json:"version"`
	LocalTimeOffset int            `json:"local_time_offset"`
}

type RestNotifyPost struct {
	PackageId    int             `json:"package_id"`
	UnitID       int             `json:"unit_id"`
	Type         string          `json:"type"`
	Context      *map[string]any `json:"context"`
	CmdTriggerId *int            `json:"trigger_cmd_id"`
}

type RestLogPost struct {
	Product string         `json:"product"`
	Level   string         `json:"level"`
	Context map[string]any `json:"context"`
}

// GET DTO -------------------------------------------------

type RestCommandGet struct {
	ID      int    `json:"id"`
	Command string `json:"command"`
}

type RestSessionGet struct {
	ID int `json:"id"`
}

type RestSoftwareUpdateGet struct {
	OldID int      `json:"old_id" groups:"api,local"`
	New   *Package `json:"new"`
}

type RestSoftwareConfigGet struct {
	Path    string `json:"path"`
	ID      int    `json:"id"`
	RawData string `json:"raw_data"`
}
