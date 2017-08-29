package util

import (
	"regexp"
	"strconv"
)

type HostInfo struct {
	Hostname  string
	Role      string
	Location  string
	ClusterID int
	ServerID  int
	ReplicaID int
}

var wellFormedRegex = regexp.MustCompile("^(.*?)(?:-(ams|sjc|sng|wdc|dal))?(?:-c(\\d+))?(?:-s(\\d+))?(?:-r(\\d+))?$")
var backupRegex = regexp.MustCompile("^backup-(dal|sjc)-(\\d+)$")
var numberedRegex = regexp.MustCompile("^([a-z]*?)(\\d+)$")

var hostInfoParsers = []func(string) (*HostInfo, bool){
	parseBuild, parseBackup, parseNumbered, parseWellFormed,
}

// GetHostInfo extracts the Role, Location, ClusterID, ServerID, and ReplicaID from the provided hostname.
func GetHostInfo(hostname string) *HostInfo {
	for _, parser := range hostInfoParsers {
		if hostInfo, ok := parser(hostname); ok {
			return hostInfo
		}
	}
	return nil
}

func parseBuild(hostname string) (*HostInfo, bool) {
	if hostname == "build-trusty" {
		return &HostInfo{Hostname: hostname, Role: "build"}, true
	}
	return nil, false
}

func parseBackup(hostname string) (*HostInfo, bool) {
	groups := backupRegex.FindStringSubmatch(hostname)
	if len(groups) == 0 {
		return nil, false
	}

	return &HostInfo{
		Hostname:  hostname,
		Role:      "backup",
		Location:  groups[1],
		ClusterID: strToInt(groups[2]),
		ServerID:  0,
		ReplicaID: 0,
	}, true
}

func parseNumbered(hostname string) (*HostInfo, bool) {
	groups := numberedRegex.FindStringSubmatch(hostname)
	if len(groups) == 0 {
		return nil, false
	}

	return &HostInfo{
		Hostname:  hostname,
		Role:      groups[1],
		Location:  "",
		ClusterID: 0,
		ServerID:  0,
		ReplicaID: strToInt(groups[2]),
	}, true
}

func parseWellFormed(hostname string) (*HostInfo, bool) {
	groups := wellFormedRegex.FindStringSubmatch(hostname)
	if len(groups) == 0 {
		return nil, false
	}

	return &HostInfo{
		Hostname:  hostname,
		Role:      groups[1],
		Location:  groups[2],
		ClusterID: strToInt(groups[3]),
		ServerID:  strToInt(groups[4]),
		ReplicaID: strToInt(groups[5]),
	}, true
}

func (info HostInfo) Map() map[string]interface{} {
	result := make(map[string]interface{}, 6)
	result["hostname"] = info.Hostname
	result["role"] = info.Role

	if len(info.Location) > 0 {
		result["location"] = info.Location
	}
	if info.ClusterID != 0 {
		result["cluster_id"] = info.ClusterID
	}
	if info.ServerID != 0 {
		result["server_id"] = info.ServerID
	}
	if info.ReplicaID != 0 {
		result["replica_id"] = info.ReplicaID
	}
	return result
}

func strToInt(str string) int {
	n, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return n
}
