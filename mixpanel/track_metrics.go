package mixpanel

import (
	"configmanager"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"obs"
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type MetricsTracker struct {
	StartTime time.Time

	fs       obs.FlightSpan
	client   Client
	success  bool
	warnType string
}

func (qm *MetricsTracker) Init(token string, url string, fs obs.FlightSpan, warnType string) {
	qm.InitWithDests([]TrackDestination{TrackDestination{TrackingToken: token, TrackingEndpoint: url}}, fs, warnType)
}

func (qm *MetricsTracker) InitWithCM(cm configmanager.Client, fs obs.FlightSpan, warnType string) {
	rand.Seed(time.Now().UnixNano())
	qm.StartTime = time.Now()
	traceClientOnce.Do(func() {
		traceClientInstance = NewClientWithConfigManager(cm)
	})
	qm.client = traceClientInstance
	qm.fs = fs
	qm.warnType = warnType
}

func (qm *MetricsTracker) InitWithDests(destinations []TrackDestination, fs obs.FlightSpan, warnType string) {
	var projects []Project
	for _, d := range destinations {
		projects = append(projects, d.Project())
	}
	rand.Seed(time.Now().UnixNano())
	qm.StartTime = time.Now()
	qm.initMetrics(projects)
	qm.fs = fs
	qm.warnType = warnType
}

type sizeWriter struct {
	size int
}

func (sw *sizeWriter) Write(p []byte) (int, error) {
	sw.size += len(p)
	return len(p), nil
}

func (qm *MetricsTracker) EstimateSize(vals obs.Vals) int {
	sw := &sizeWriter{}
	json.NewEncoder(sw).Encode(vals)
	return sw.size
}

func (qm *MetricsTracker) ProcessError(msg string, err error) {
	if err == nil {
		qm.fs.Warn(qm.warnType, msg, obs.Vals{})
	} else {
		qm.fs.Warn(qm.warnType, msg, obs.Vals{}.WithError(err))
	}
}

func (qm *MetricsTracker) ProcessInfo(msg string, vals obs.Vals) {
	qm.fs.Info(msg, vals)
}

func (qm *MetricsTracker) initMetrics(projects []Project) {
	if len(projects) > 0 && len(projects[0].Token) > 0 && len(projects[0].BaseUrl) > 0 {
		traceClientOnce.Do(func() {
			traceClientInstance = NewClientWithProjects(projects) // No API key is needed for tracking
		})
	}
	qm.client = traceClientInstance
}

func (qm *MetricsTracker) Fail() {
	qm.success = false
}

func (qm *MetricsTracker) Succeed() {
	qm.success = true
}

func (qm *MetricsTracker) Success() bool {
	return qm.success
}

func (qm *MetricsTracker) IncrCounter(c string) {
	qm.fs.Incr(c)
}

// trackQuery completes the trackedevent and sends the tracking call
func (qm *MetricsTracker) TrackQuery(ev *TrackedEvent) {
	if ev != nil {
		ev.Properties["success"] = qm.success

		if qm.client != nil {
			err := qm.client.Track(ev)
			if err != nil {
				qm.ProcessError("TrackQuery failed", err)
			}
		}
	} else {
		qm.ProcessError("TrackEvent creation failed", nil)
	}
}

func generateMap(q interface{}, translation map[string]string, m map[string]interface{}) {
	v := reflect.Indirect(reflect.ValueOf(q))
	if v.Kind() != reflect.Struct {
		return
	}
	t := reflect.TypeOf(q)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < v.NumField(); i++ {
		fn := t.Field(i).Name
		fv := reflect.Indirect(v.Field(i))
		if mn, ok := translation[fn]; ok {
			m[mn] = fv.Interface()
		}
	}
}

func GenerateStringKVMap(q interface{}, translation map[string]string) map[string]string {
	m := make(map[string]string)
	v := reflect.Indirect(reflect.ValueOf(q))
	if v.Kind() != reflect.Struct {
		return m
	}

	t := reflect.TypeOf(q)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		fn := t.Field(i).Name
		fv := reflect.Indirect(v.Field(i))
		switch fv.Kind() {
		case reflect.Uint64:
			m[fn] = strconv.FormatUint(fv.Interface().(uint64), 10)
		case reflect.Int64:
			m[fn] = strconv.FormatInt(fv.Interface().(int64), 10)
		case reflect.Float64:
			m[fn] = strconv.FormatFloat(fv.Interface().(float64), 'f', -1, 64)
		case reflect.Bool:
			m[fn] = strconv.FormatBool(fv.Interface().(bool))
		case reflect.String:
			m[fn] = fv.Interface().(string)
		default:
		}
	}

	return m
}

func GenerateMapTo(qm interface{}, translation map[string]string, m map[string]interface{}) {
	if m == nil {
		m = make(map[string]interface{})
	}

	generateMap(qm, translation, m)
}

func setField(q interface{}, name string, value string) error {
	qv := reflect.Indirect(reflect.ValueOf(q))
	if !qv.CanSet() || qv.Kind() != reflect.Struct {
		return grpc.Errorf(codes.InvalidArgument, "track_metrics: %#v is not a settable struct. can set: %t kind: %v", q, qv.CanSet(), qv.Kind())
	}
	field := reflect.Indirect(qv.FieldByName(name))

	if !field.IsValid() {
		return fmt.Errorf("No such field: %s in obj", name)
	}

	if !field.CanSet() {
		return fmt.Errorf("Cannot set %s field value", name)
	}

	var val reflect.Value
	kind := field.Type().Kind()
	switch kind {
	case reflect.Uint64:
		n, _ := strconv.ParseUint(value, 10, 64)
		val = reflect.ValueOf(n)
	case reflect.Int64:
		n, _ := strconv.ParseInt(value, 10, 64)
		val = reflect.ValueOf(n)
	case reflect.Float64:
		n, _ := strconv.ParseFloat(value, 64)
		val = reflect.ValueOf(n)
	case reflect.String:
		val = reflect.ValueOf(value)
	case reflect.Bool:
		n, _ := strconv.ParseBool(value)
		val = reflect.ValueOf(n)
	default:
		return errors.New("Provided value type didn't match obj field type")
	}

	field.Set(val)

	return nil
}

func FillFromMap(q interface{}, m map[string]string) error {
	t := reflect.TypeOf(q)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		originalKey := t.Field(i).Name
		k := strings.ToLower(originalKey)
		if v, ok := m[k]; ok {
			err := setField(q, originalKey, v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
