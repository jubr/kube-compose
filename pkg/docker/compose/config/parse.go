package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kube-compose/kube-compose/internal/pkg/util"
	"github.com/uber-go/mapdecode"
)

type stringOrStringSlice struct {
	Values []string
}

func (t *stringOrStringSlice) Decode(into mapdecode.Into) error {
	err := into(&t.Values)
	if err != nil {
		var str string
		err = into(&str)
		if err != nil {
			return err
		}
		t.Values = []string{str}
	}
	return nil
}

type HealthcheckTest struct {
	Values []string
}

func (t *HealthcheckTest) Decode(into mapdecode.Into) error {
	err := into(&t.Values)
	if err != nil {
		var str string
		err = into(&str)
		if err != nil {
			return err
		}
		t.Values = []string{
			HealthcheckCommandShell,
			str,
		}
	}
	return nil
}

type healthcheckInternal struct {
	Disable  *bool   `mapdecode:"disable"`
	Interval *string `mapdecode:"interval"`
	Retries  *uint   `mapdecode:"retries"`
	// Test.Values is nil if and only if the field "test" is not present in the map.
	// If the field "test" is present and is an empty slice, then Test.Values will not be nil.
	Test    HealthcheckTest `mapdecode:"test"`
	Timeout *string         `mapdecode:"timeout"`
	// start_period is only available in docker-compose 2.3 or higher
}

func (h *healthcheckInternal) IsEmpty() bool {
	return h.Disable == nil && h.Interval == nil && h.Retries == nil && h.GetTest() == nil && h.Timeout == nil
}

func (h *healthcheckInternal) GetTest() []string {
	return h.Test.Values
}

type dependsOn struct {
	Values map[string]ServiceHealthiness
}

func (t *dependsOn) Decode(into mapdecode.Into) error {
	var strMap map[string]struct {
		Condition string `mapdecode:"condition"`
	}
	err := into(&strMap)
	if err != nil {
		var services []string
		err = into(&services)
		if err != nil {
			return err
		}
		t.Values = map[string]ServiceHealthiness{}
		for _, service := range services {
			_, ok := t.Values[service]
			if ok {
				return fmt.Errorf("depends_on list cannot contain duplicate values")
			}
			t.Values[service] = ServiceStarted
		}
	} else {
		n := len(strMap)
		t.Values = make(map[string]ServiceHealthiness, n)
		for service, obj := range strMap {
			switch obj.Condition {
			case "service_healthy":
				t.Values[service] = ServiceHealthy
			case "service_started":
				t.Values[service] = ServiceStarted
			case "service_completed_successfully":
				t.Values[service] = ServiceCompletedSuccessfully
			default:
				return fmt.Errorf("depends_on map contains an entry with an invalid condition: %s", obj.Condition)
			}
		}
	}
	return nil
}

type environmentNameValuePair struct {
	Name  string
	Value *environmentValue
}

// TODO https://github.com/kube-compose/kube-compose/issues/40 check whether handling of large numbers is consistent with docker-compose
// See https://github.com/docker/compose/blob/master/compose/config/config_schema_v2.1.json#L418
type environmentValue struct {
	FloatValue  *float64
	Int64Value  *int64
	StringValue *string
}

func (v *environmentValue) Decode(into mapdecode.Into) error {
	var f float64
	err := into(&f)
	if err == nil {
		if -9223372036854775000.0 <= f && f <= 9223372036854775000.0 && math.Floor(f) == f {
			v.Int64Value = new(int64)
			*v.Int64Value = int64(f)
			return nil
		}
		v.FloatValue = new(float64)
		*v.FloatValue = f
		return nil
	}
	var s string
	err = into(&s)
	if err == nil {
		v.StringValue = util.NewString(s)
	}
	return err
}

type environment struct {
	Values []environmentNameValuePair
}

func (t *environment) Decode(into mapdecode.Into) error {
	var intoMap map[string]environmentValue
	err := into(&intoMap)
	if err == nil {
		i := 0
		t.Values = make([]environmentNameValuePair, len(intoMap))
		for name, value := range intoMap {
			t.Values[i].Name = name
			valueCopy := new(environmentValue)
			*valueCopy = value
			t.Values[i].Value = valueCopy
			i++
		}
		return nil
	}
	var intoSlice []string
	err = into(&intoSlice)
	if err == nil {
		t.Values = make([]environmentNameValuePair, len(intoSlice))
		for i, nameValuePair := range intoSlice {
			j := strings.IndexRune(nameValuePair, '=')
			if j < 0 {
				t.Values[i].Name = nameValuePair
			} else {
				t.Values[i].Name = nameValuePair[:j]
				stringValue := nameValuePair[j+1:]
				t.Values[i].Value = &environmentValue{
					StringValue: &stringValue,
				}
			}
		}
	}
	return err
}

type extendsHelper struct {
	File    *string `mapdecode:"file"`
	Service string  `mapdecode:"service"`
}

type extends struct {
	File    *string
	Service string
}

// Used by mapdecode package
func (e *extends) Decode(into mapdecode.Into) error {
	var serviceName string
	err := into(&serviceName)
	if err == nil {
		e.Service = serviceName
		return nil
	}
	var eHelper extendsHelper
	err = into(&eHelper)
	if err != nil {
		return err
	}
	e.File = eHelper.File
	e.Service = eHelper.Service
	return nil
}

type port struct {
	Value string
}

func (p *port) Decode(into mapdecode.Into) error {
	var int64Val int64
	err := into(&int64Val)
	if err == nil {
		p.Value = strconv.FormatInt(int64Val, 10)
		return nil
	}
	strVal := ""
	err = into(&strVal)
	p.Value = strVal
	return err
}

// ServiceVolume is the type used to encode each volume of a docker compose service.
type ServiceVolume struct {
	Short *PathMapping
}

// Decode parses either the long or short syntax of a docker-compose service volume into the ServiceVolume type.
func (sv *ServiceVolume) Decode(into mapdecode.Into) error {
	var shortSyntax string
	err := into(&shortSyntax)
	if err == nil {
		sv.Short = &PathMapping{}
		*sv.Short = parsePathMapping(shortSyntax)
		return nil
	}
	// TODO https://github.com/kube-compose/kube-compose/issues/161 support long volume syntax
	return err
}
