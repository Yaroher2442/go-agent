package helpers

import (
	"bytes"
	"gopkg.in/ini.v1"
	"net"
	"os"
	"os/exec"
	"os/user"
)

func ShellOutCaptureErr(command string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}
func ShellOutCaptureOutErr(command string) (string, string, error) {
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func GetLocalIP() net.IP {
	ip := func() net.IP {
		conn, err := net.Dial("udp", "8.8.8.8:80")
		if err != nil {
			return nil
		}
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP
	}()
	if ip == nil {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP
				}
			}
		}
	} else {
		return ip
	}
	return nil
}

func IsRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, err
	}
	return currentUser.Username == "root", nil
}

type LinuxOsReleaseInfo struct {
	PRETTY_NAME        string
	NAME               string
	VERSION_ID         string
	VERSION            string
	VERSION_CODENAME   string
	ID                 string
	ID_LIKE            string
	HOME_URL           string
	SUPPORT_URL        string
	BUG_REPORT_URL     string
	PRIVACY_POLICY_URL string
	UBUNTU_CODENAME    string
}

func GetLinuxOSRelease(getFrom *string) (*LinuxOsReleaseInfo, error) {
	var getDir string
	if getFrom == nil || *getFrom == "" {
		getDir = "/etc/os-release"
	}
	cfg, err := ini.Load(getDir)
	if err != nil {
		return nil, err
	}
	return &LinuxOsReleaseInfo{
		PRETTY_NAME:        cfg.Section("").Key("PRETTY_NAME").String(),
		NAME:               cfg.Section("").Key("NAME").String(),
		VERSION_ID:         cfg.Section("").Key("VERSION_ID").String(),
		VERSION:            cfg.Section("").Key("VERSION").String(),
		VERSION_CODENAME:   cfg.Section("").Key("VERSION_CODENAME").String(),
		ID:                 cfg.Section("").Key("ID").String(),
		ID_LIKE:            cfg.Section("").Key("ID_LIKE").String(),
		HOME_URL:           cfg.Section("").Key("HOME_URL").String(),
		SUPPORT_URL:        cfg.Section("").Key("SUPPORT_URL").String(),
		BUG_REPORT_URL:     cfg.Section("").Key("BUG_REPORT_URL").String(),
		PRIVACY_POLICY_URL: cfg.Section("").Key("PRIVACY_POLICY_URL").String(),
		UBUNTU_CODENAME:    cfg.Section("").Key("UBUNTU_CODENAME").String(),
	}, nil
}
