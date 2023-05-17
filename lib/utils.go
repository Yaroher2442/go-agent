package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/liip/sheriff"
	"main/lib/helpers"
	"main/lib/log"
	"main/lib/structs"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Go

func WithLock(lock *sync.Mutex, f func() error) func() error {
	return func() error {
		canLock := lock.TryLock()
		if canLock == false {
			return errors.New(" WithLock() fails, can't acquire given lock ")
		}
		log.Log.Debug().Msg("Locked()")
		defer lock.Unlock()
		defer log.Log.Debug().Msg("UnLocked()")
		err := f()
		return err
	}
}

func MergeMaps(src ...map[string]any) *map[string]any {
	merged := make(map[string]any)
	for _, m := range src {
		for k, v := range m {
			merged[k] = v
		}
	}
	return &merged
}

func CreateRpcConn() *rpc.Client {
	conn, conerr := net.DialTimeout("tcp", RrcDns, 1*time.Second)
	if conerr != nil {
		log.Log.Debug().Msg("Connect to service failed, is pca.service alive?")
	} else {
		return rpc.NewClient(conn)
	}
	return nil
}

// Formats

func VersionStringColor(instSoft *structs.Software, newSoft *structs.Software) string {
	str := fmt.Sprintf("%s -> %s",
		instSoft.Branch+"_"+instSoft.Version+"+"+strconv.Itoa(instSoft.Patch),
		newSoft.String())
	ol, _ := version.NewVersion(instSoft.Version)
	nw, _ := version.NewVersion(newSoft.Version)
	if ol.Equal(nw) {
		if instSoft.Patch == newSoft.Patch {
			str = text.FgBlack.Sprint(str)
		} else if instSoft.Patch < newSoft.Patch {
			str = text.FgGreen.Sprint(str)
		} else if instSoft.Patch > newSoft.Patch {
			str = text.FgMagenta.Sprint(str)
		}
	} else {
		if nw.GreaterThanOrEqual(ol) {
			str = text.FgGreen.Sprint(str)
		} else {
			str = text.FgYellow.Sprint(str)
		}
	}
	return str
}

func ColorKind(kind string) string {
	var kindText string
	switch kind {
	case "server":
		kindText = text.FgBlue.Sprint(kind)
	case "front":
		kindText = text.FgCyan.Sprint(kind)
	case "terminal":
		kindText = text.FgMagenta.Sprint(kind)
	case "driver":
		kindText = text.FgHiBlack.Sprint(kind)
	case "application":
		kindText = text.FgYellow.Sprint(kind)
	}
	return kindText
}

// Os info

func OsVersion() string {
	osRealease, _ := helpers.GetLinuxOSRelease(nil)
	versionId := strings.ToLower(strings.Split(osRealease.VERSION_ID, ".")[0])
	return fmt.Sprintf("%s:%s", strings.ToLower(osRealease.NAME), versionId)
}

func FindInstalledEditor() string {
	editor := "vim"
	for _, maybe := range []string{"micro", "nano"} {
		_, err := exec.LookPath(maybe)
		if err != nil {
			continue
		} else {
			editor = maybe
			break
		}
	}
	return editor
}

// Apis

func WriteJsonResponse(w http.ResponseWriter, statusCode int, data map[string]any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		return err
	}
	return err
}

// Files

func SafeReadJsonFile(filename string, v interface{}) error {
	fileLock := helpers.MakeFileMutex(filename)
	fileLock.Lock()
	defer fileLock.Unlock()
	fp, err := os.Open(filename)
	err = json.NewDecoder(fp).Decode(v)
	return err
}

func SafeWriteJsonFile(v interface{}, sheriffOpts *sheriff.Options, filename string, fileMode os.FileMode) error {
	fileLock := helpers.MakeFileMutex(filename)
	fileLock.Lock()
	defer fileLock.Unlock()
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
	defer f.Close()
	if err != nil {
		return err
	}
	if sheriffOpts != nil {
		v, err = sheriff.Marshal(sheriffOpts, v)
		if err != nil {
			return err
		}
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// Agent asks

func AskInput(selectName string, validate func(input string) bool) {
	for {
		var input string
		log.Log.Info().Msgf("Please choose %s: ", selectName)
		_, err := fmt.Scanln(&input)
		if err != nil || input == "" {
			log.Log.Warn().Msg("Filed to scan from console")
		} else {
			if validate(input) {
				break
			}
		}
	}
}

func AskConfirm(force *bool) bool {
	confirm := false
	if *force {
		confirm = true
	} else {
		AskInput("(y/n)", func(input string) bool {
			if input == "y" {
				confirm = true
				return true
			} else if input == "n" {
				confirm = false
				return true
			}
			log.Log.Info().Msg("Chose from (y/n)\n")
			return false
		})
	}
	return confirm
}

func AskTargetClient(clientId *int, clients []*structs.Client) *structs.Client {
	if clientId != nil {
		for num, client := range clients {
			if client.ID == *clientId {
				row := table.Row{strconv.Itoa(num + 1)}
				row = append(row, *client.Row()...)
				t := helpers.ConstructTable(client.Header())
				t.AppendRow(row)
				t.Render()
				log.Log.Info().Msgf("Auto chose %d", *clientId)
				return client
			}
		}
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ID < clients[j].ID
	})
	var targetClient *structs.Client
	if len(clients) == 0 {
		return nil
	}
	t := helpers.ConstructTable(clients[0].Header())
	for num, c := range clients {
		row := table.Row{strconv.Itoa(num + 1)}
		row = append(row, *c.Row()...)
		t.AppendRow(row)
		t.AppendSeparator()
	}
	t.Render()
	AskInput("client id", func(input string) bool {
		num, err := strconv.Atoi(input)
		if err != nil {
			log.Log.Warn().Msg("Pass only digit value value, please retry")
			return false
		}
		if num > len(clients) || num == 0 {
			log.Log.Warn().Msg("This client not exists, please retry")
			return false
		} else {
			targetClient = clients[num-1]
			return true
		}
	})
	return targetClient
}

func AskTargetProduct(productId *int, products []*structs.Product) *structs.Product {
	if productId != nil {
		for num, product := range products {
			if product.ID == *productId {
				row := table.Row{strconv.Itoa(num + 1)}
				row = append(row, *product.Row()...)
				t := helpers.ConstructTable(product.Header())
				t.AppendRow(row)
				t.Render()
				log.Log.Info().Msgf("Auto chose %d", *productId)
				return product
			}
		}
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].ID < products[j].ID
	})
	var targetProduct *structs.Product
	if len(products) == 0 {
		return nil
	}
	t := helpers.ConstructTable(products[0].Header())
	for num, product := range products {
		row := table.Row{strconv.Itoa(num + 1)}
		row = append(row, *product.Row()...)
		t.AppendRow(row)
		t.AppendSeparator()
	}
	t.Render()
	AskInput("products id", func(input string) bool {
		num, err := strconv.Atoi(input)
		if err != nil {
			log.Log.Warn().Msg("Pass only digit value value, please retry")
			return false
		}
		if num > len(products) || num == 0 {
			log.Log.Warn().Msg("This product not exists, please retry")
			return false
		} else {
			targetProduct = products[num-1]
			return true
		}
	})
	return targetProduct
}

func AskTargetPackage(product *structs.Product, units []*structs.Unit, packageId *int) (*structs.Unit, *structs.Package) {
	log.Log.Info().Msgf("Fetched %d units", len(units))
	if packageId != nil {
		for _, u := range units {
			for _, p := range u.Packages {
				if p.ID == *packageId {
					//log.Log.Info().Msg("Install this:")
					//l := list.NewWriter()
					//l.SetStyle(list.StyleConnectedRounded)
					//p.DrawInList(l)
					//printList("\nInstallation software", l.Render(), "")
					for _, pi := range p.PackageItems {
						if !pi.Enable {
							return nil, nil
						}
					}
					return u, p
				}
			}
		}
	}
	if len(units) == 0 {
		log.Log.Warn().Msgf("Nothing to install in %s", product.Name)
		return nil, nil
	}
	t := helpers.ConstructTable(&table.Row{"#", "Name", "Description"})
	//zeroUnitPackages := units[0].Packages
	var packages []structs.Package
	var idx = 0
	for _, unit := range units {
		for _, pkg := range unit.Packages {
			t.AppendRow(table.Row{idx + 1,
				fmt.Sprintf("%s\n(unit id:%d; package id: %d)", pkg.Name, unit.ID, pkg.ID), pkg.Description})
			t.AppendSeparator()
			packages = append(packages, *pkg)
			idx++
		}
	}
	t.Render()
	var targetPackageName string
	AskInput("package name num", func(input string) bool {
		num, err := strconv.Atoi(input)
		if num > len(packages) || num == 0 || err != nil {
			log.Log.Warn().Msg("This package not exists, please retry")
			return false
		} else {
			targetPackageName = packages[num-1].Name
			return true
		}
	})
	pkgList := make([]*structs.Package, 0)
	for _, unit := range units {
		pkg := helpers.Find(unit.Packages, func(p *structs.Package) bool {
			if p.Name == targetPackageName {
				return true
			}
			return false
		})
		if pkg != nil {
			pkgList = append(pkgList, *pkg)
		}
	}

	var targetPackage *structs.Package

	sort.Slice(pkgList, func(i, j int) bool {
		return pkgList[i].ID > pkgList[j].ID
	})
	t = helpers.ConstructTable(&table.Row{"#", "Package", "Software", "Kind", "Version", "Enable"})
	for num, pkg := range pkgList {
		t.AppendRow(table.Row{num + 1, pkg.Name})
		for _, pi := range pkg.PackageItems {
			enableText := text.FgRed.Sprint("Not enable on this system")
			if pi.Enable {
				enableText = text.FgGreen.Sprint("Enable on this system")
			}
			t.AppendRow(table.Row{"", "",
				pi.Software.Name,
				ColorKind(pi.Software.Kind),
				pi.Software.String(),
				enableText})
		}
		t.AppendSeparator()
	}
	t.Render()
	AskInput("package id", func(input string) bool {
		num, err := strconv.Atoi(input)
		if err != nil {
			log.Log.Warn().Msg("Please pass int")
			return false
		}
		if num != 0 && num <= len(pkgList) {
			targetPackage = pkgList[num-1]
			return true
		}
		log.Log.Warn().Msg("Num of package not exists, please retry")
		return false
	})
	//log.Log.Info().Msg("Install this:")
	//l = list.NewWriter()
	//l.SetStyle(list.StyleConnectedRounded)
	//(*targetPackage).DrawInList(l)
	//printList("\nInstallation software", l.Render(), "")
	targetUnit := helpers.Find(units, func(unit *structs.Unit) bool {
		for _, packages := range unit.Packages {
			if packages.ID == (*targetPackage).ID {
				return true
			}
		}
		return false
	})
	return *targetUnit, targetPackage
}
