// +build !nogmond

package collector

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"regexp"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/node_exporter/collector/ganglia"
)

const (
	gangliaAddress   = "127.0.0.1:8649"
	gangliaProto     = "tcp"
	gangliaTimeout   = 30 * time.Second
	gangliaNamespace = "ganglia"
)

type gmondCollector struct {
	metrics map[string]*prometheus.GaugeVec
}

func init() {
	Factories["gmond"] = NewGmondCollector
}

var illegalCharsRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// Takes a prometheus registry and returns a new Collector scraping ganglia.
func NewGmondCollector() (Collector, error) {
	c := gmondCollector{
		metrics: map[string]*prometheus.GaugeVec{},
	}

	return &c, nil
}

func (c *gmondCollector) Update(ch chan<- prometheus.Metric) (err error) {
	conn, err := net.Dial(gangliaProto, gangliaAddress)
	glog.V(1).Infof("gmondCollector Update")
	if err != nil {
		return fmt.Errorf("Can't connect to gmond: %s", err)
	}
	conn.SetDeadline(time.Now().Add(gangliaTimeout))

	ganglia := ganglia.Ganglia{}
	decoder := xml.NewDecoder(bufio.NewReader(conn))
	decoder.CharsetReader = toUtf8

	err = decoder.Decode(&ganglia)
	if err != nil {
		return fmt.Errorf("Couldn't parse xml: %s", err)
	}

	for _, cluster := range ganglia.Clusters {
		for _, host := range cluster.Hosts {

			for _, metric := range host.Metrics {
				name := illegalCharsRE.ReplaceAllString(metric.Name, "_")

				c.setMetric(name, cluster.Name, metric)
			}
		}
	}
	for _, m := range c.metrics {
		m.Collect(ch)
	}
	return err
}

func (c *gmondCollector) setMetric(name, cluster string, metric ganglia.Metric) {
	if _, ok := c.metrics[name]; !ok {
		var desc string
		var title string
		for _, element := range metric.ExtraData.ExtraElements {
			switch element.Name {
			case "DESC":
				desc = element.Val
			case "TITLE":
				title = element.Val
			}
			if title != "" && desc != "" {
				break
			}
		}
		glog.V(1).Infof("Register %s: %s", name, desc)
		c.metrics[name] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: gangliaNamespace,
				Name:      name,
				Help:      desc,
			},
			[]string{"cluster"},
		)
	}
	glog.V(1).Infof("Set %s{cluster=%q}: %f", name, cluster, metric.Value)
	c.metrics[name].WithLabelValues(cluster).Set(metric.Value)
}

func toUtf8(charset string, input io.Reader) (io.Reader, error) {
	return input, nil //FIXME
}
