package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostInfo(t *testing.T) {
	testCases := map[string]HostInfo{
		"aliases-s01-r02":           {"", "aliases", "", 0, 1, 2},
		"api-arb-sjc-r03":           {"", "api-arb", "sjc", 0, 0, 3},
		"api-arb-wdc-r04":           {"", "api-arb", "wdc", 0, 0, 4},
		"api-arb-sng-r04":           {"", "api-arb", "sng", 0, 0, 4},
		"api-decide-sjc-r06":        {"", "api-decide", "sjc", 0, 0, 6},
		"api-engage-sjc-r07":        {"", "api-engage", "sjc", 0, 0, 7},
		"api-queue-arb-sjc-r08":     {"", "api-queue-arb", "sjc", 0, 0, 8},
		"api-queue-engage-sjc-r09":  {"", "api-queue-engage", "sjc", 0, 0, 9},
		"app01":                     {"", "app", "", 0, 0, 1},
		"appdb-r01":                 {"", "appdb", "", 0, 0, 1},
		"appdb-readonly":            {"", "appdb-readonly", "", 0, 0, 0},
		"arb-data-c01-s01-r01":      {"", "arb-data", "", 1, 1, 1},
		"arb-query-c01-r01":         {"", "arb-query", "", 1, 0, 1},
		"arb-queue-r01":             {"", "arb-queue", "", 0, 0, 1},
		"backup-dal-15":             {"", "backup", "dal", 15, 0, 0},
		"backup-sjc-01":             {"", "backup", "sjc", 1, 0, 0},
		"build-trusty":              {"", "build", "", 0, 0, 0},
		"cache01":                   {"", "cache", "", 0, 0, 1},
		"consumer-r001":             {"", "consumer", "", 0, 0, 1},
		"consumer-r101":             {"", "consumer", "", 0, 0, 101},
		"cron-r01":                  {"", "cron", "", 0, 0, 1},
		"engage-queue-r01":          {"", "engage-queue", "", 0, 0, 1},
		"export-r01":                {"", "export", "", 0, 0, 1},
		"exportstage-r01":           {"", "exportstage", "", 0, 0, 1},
		"logstash-r01":              {"", "logstash", "", 0, 0, 1},
		"mail02":                    {"", "mail", "", 0, 0, 2},
		"mailconsole":               {"", "mailconsole", "", 0, 0, 0},
		"monitor01":                 {"", "monitor", "", 0, 0, 1},
		"notifications-batcher-r01": {"", "notifications-batcher", "", 0, 0, 1},
		"notifications-queue-r01":   {"", "notifications-queue", "", 0, 0, 1},
		"redis-s01":                 {"", "redis", "", 0, 1, 0},
		"saleseng01":                {"", "saleseng", "", 0, 0, 1},
		"support-team":              {"", "support-team", "", 0, 0, 0},
		"switchboard01":             {"", "switchboard", "", 0, 0, 1},
		"trends01":                  {"", "trends", "", 0, 0, 1},
	}

	for host, expected := range testCases {
		expected.Hostname = host
		actual := GetHostInfo(host)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, *actual)
	}
}

func TestHostInfoMap(t *testing.T) {
	hostInfo := HostInfo{"a-hostname", "a-role", "sjc", 1, 2, 3}
	expected := map[string]interface{}{
		"hostname":   "a-hostname",
		"role":       "a-role",
		"location":   "sjc",
		"cluster_id": 1,
		"server_id":  2,
		"replica_id": 3,
	}

	assert.Equal(t, expected, hostInfo.Map())
}

func TestHostInfoMapWithMissingFields(t *testing.T) {
	hostInfo := HostInfo{"a-hostname", "a-role", "", 0, 0, 0}
	expected := map[string]interface{}{
		"hostname": "a-hostname",
		"role":     "a-role",
	}

	assert.Equal(t, expected, hostInfo.Map())
}
