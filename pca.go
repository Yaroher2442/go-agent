package main

import (
	"main/lib"
	"main/lib/log"
	"os"
)

func main() {
	//lib.HandleRoot()
	lib.FindPackageSystem()
	lib.ConfigurePaths()
	settings := lib.LoadSettings()
	log.Log.Debug().Msgf("Use package manager: %s", lib.RunPkgManager)
	log.Log.Debug().Msgf("Agent path is %s", "/opt/abt/pca")
	apiClient := lib.NewRestClient(settings)
	rpcClient := lib.CreateRpcConn()
	agent := lib.NewAgent(settings, apiClient, rpcClient)
	err := lib.SelectCommand(os.Args[1:], agent)
	if err != nil {
		log.Log.Error().Err(err)
	}
}
