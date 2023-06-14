package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
)

const (
	program = "s3_ceph_exporter"
)

type S3Collector struct {
	sizeActualMetric      *prometheus.Desc
	totalSizeActualMetric *prometheus.Desc
	sizeUtilizedMetric    *prometheus.Desc
	numObjectsMetric      *prometheus.Desc
	cephAccessKey         string
	cephSecretKey         string
	cephGatewayURL        string
	bucketStats           map[string]BucketStats
}

type BucketStats struct {
	Name  string       `json:"bucket"`
	Usage UsageDetails `json:"usage"`
}

type UsageDetails struct {
	SizeDetails UsageSizeDetails `json:"rgw.main"`
}

type UsageSizeDetails struct {
	SizeActual   int `json:"size_kb_actual"`
	SizeUtilized int `json:"size_kb_utilized"`
	NumObjects   int `json:"num_objects"`
}

func (collector *S3Collector) updateBucketStats() error {
	// Generate the timestamp and date in UTC
	date := time.Now().UTC().Format(time.RFC1123)

	// Generate the string to sign
	stringToSign := fmt.Sprintf("GET\n\n\n%s\n/admin/bucket", date)
	hmac := hmac.New(sha1.New, []byte(collector.cephSecretKey))
	hmac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(hmac.Sum(nil))

	// Send the request
	client := &http.Client{}
	req, err := http.NewRequest("GET", collector.cephGatewayURL+"/admin/bucket?stats", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Host", strings.Split(req.URL.Host, ":")[0])
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", collector.cephAccessKey, signature))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Read the response
	var buckets []BucketStats
	err = json.NewDecoder(res.Body).Decode(&buckets)
	if err != nil {
		return err
	}

	// Update the bucket stats map
	collector.bucketStats = make(map[string]BucketStats)
	for _, bucket := range buckets {
		collector.bucketStats[bucket.Name] = bucket
	}

	return nil
}

func newS3Collector(cephAccessKey, cephSecretKey, cephGatewayURL string) *S3Collector {
	return &S3Collector{
		sizeActualMetric: prometheus.NewDesc("s3_bucket_actual_size",
			"s3 bucket size",
			[]string{"name"}, nil,
		),
		totalSizeActualMetric: prometheus.NewDesc("s3_bucket_total_size",
			"s3 total buckets size",
			[]string{"category"}, nil,
		),
		sizeUtilizedMetric: prometheus.NewDesc("s3_bucket_utilized_size",
			"s3 bucket utilized size",
			[]string{"name"}, nil,
		),
		numObjectsMetric: prometheus.NewDesc("s3_bucket_num_objects",
			"s3 bucket number of objects",
			[]string{"name"}, nil,
		),
		cephAccessKey:  cephAccessKey,
		cephSecretKey:  cephSecretKey,
		cephGatewayURL: cephGatewayURL,
		bucketStats:    make(map[string]BucketStats),
	}
}

func (collector *S3Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.sizeActualMetric
	ch <- collector.totalSizeActualMetric
	ch <- collector.numObjectsMetric
	ch <- collector.numObjectsMetric
}

func (collector *S3Collector) Collect(ch chan<- prometheus.Metric) {

	err := collector.updateBucketStats()
	if err != nil {
		log.Printf("Error updating bucket stats: %s", err)
		return
	}

	// Collect the metrics
	for bucketName, stats := range collector.bucketStats {
		ch <- prometheus.MustNewConstMetric(collector.sizeActualMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.SizeActual), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.sizeUtilizedMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.SizeUtilized), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.numObjectsMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.NumObjects), bucketName)
	}

	// // Calculate total size across all buckets
	var totalSize float64
	for _, stats := range collector.bucketStats {
		totalSize += float64(stats.Usage.SizeDetails.SizeActual)
	}

	sizes := map[string]float64{
		"KB": totalSize,
		"MB": totalSize / 1024,
		"GB": totalSize / (1024 * 1024),
		"TB": totalSize / (1024 * 1024 * 1024),
	}

	for unit, size := range sizes {
		ch <- prometheus.MustNewConstMetric(
			collector.totalSizeActualMetric,
			prometheus.GaugeValue,
			size,
			unit,
		)
	}
}

func getEnv(key string, defaultVal string) string {
	if env, ok := os.LookupEnv(key); ok {
		return env
	}
	return defaultVal
}

func init() {
	prometheus.MustRegister(version.NewCollector(program))
}

func main() {
	var (
		printVersion      = flag.Bool("version", false, "Print version information.")
		listenAddress     = flag.String("web.listen-address", getEnv("LISTEN_ADDRESS", ":9290"), "Address to listen on for web interface and telemetry.")
		metricsPath       = flag.String("web.telemetry-path", getEnv("METRIC_PATH", "/metrics"), "Path under which to expose metrics.")
		cephRadosgwURI    = flag.String("radosgw.server", getEnv("CEPH_URL", "http://localhost:9000"), "HTTP address of the ceph radosgw server")
		cephRadosgwKey    = flag.String("radosgw.access-key", getEnv("CEPH_ACCESS_KEY", ""), "The access key used to login in to ceph radosgw.")
		cephRadosgwSecret = flag.String("radosgw.access-secret", getEnv("CEPH_ACCESS_SECRET", ""), "The access secret used to login in to ceph radosgw")
	)

	flag.Parse()

	if *printVersion {
		fmt.Fprintln(os.Stdout, version.Print("s3_ceph_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting s3_ceph_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	s3 := newS3Collector(*cephRadosgwKey, *cephRadosgwSecret, *cephRadosgwURI)
	prometheus.MustRegister(s3)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
                        <head><title>s3-ceph Exporter</title></head>
                        <body>
                        <h1>s3-ceph Exporter</h1>
                        <p><a href='` + *metricsPath + `'>Metrics</a></p>
                        </body>
                        </html>`))
	})

	log.Infoln("Listening on", *listenAddress)

	err := http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}
