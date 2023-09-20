package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
)

const (
	program = "s3_ceph_exporter"
)

type S3Collector struct {
	sizeActualMetric           *prometheus.Desc
	totalUsageSizeActualMetric *prometheus.Desc
	sizeUtilizedMetric         *prometheus.Desc
	numObjectsMetric           *prometheus.Desc
	perUserUsageMetric         *prometheus.Desc
	bucketQuotaMaxSize         *prometheus.Desc
	bucketQuotaMaxObjects      *prometheus.Desc
	bucketNumShards            *prometheus.Desc
	userQuotaMaxSize           *prometheus.Desc
	userQuotaMaxObjects        *prometheus.Desc
	cephAccessKey              string
	cephSecretKey              string
	cephGatewayURL             string
	bucketStats                map[string]BucketStats
	userStats                  UserStats
}

type UserStats struct {
	MaxSize    float64 `json:"max_size"`
	MaxObjects float64 `json:"max_objects"`
}

type BucketStats struct {
	Name        string             `json:"bucket"`
	Usage       UsageDetails       `json:"usage"`
	BucketQuota BucketQuotaDetails `json:"bucket_quota"`
	BucketOwner string             `json:"owner"`
	NumShards   int                `json:"num_shards"`
}

type BucketQuotaDetails struct {
	MaxSize    int `json:"max_size"`
	MaxObjects int `json:"max_objects"`
}

type UsageDetails struct {
	SizeDetails UsageSizeDetails `json:"rgw.main"`
}

type UsageSizeDetails struct {
	SizeActual   int `json:"size_kb_actual"`
	SizeUtilized int `json:"size_kb_utilized"`
	NumObjects   int `json:"num_objects"`
}

/*
##########################################
########## Helper functions ##############
##########################################
*/

func generateSignature(method, path, secretKey string) string {
	stringToSign := fmt.Sprintf("%s\n\n\n%s\n%s", method, time.Now().UTC().Format(time.RFC1123), path)
	hmac := hmac.New(sha1.New, []byte(secretKey))
	hmac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(hmac.Sum(nil))
}

func createRequest(url, accessKey, signature string) (*http.Request, error) {
	// Generate the timestamp and date in UTC
	date := time.Now().UTC().Format(time.RFC1123)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Host", strings.Split(req.URL.Host, ":")[0])
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", accessKey, signature))
	return req, nil
}

func decodeResponse(responseBody io.Reader, target interface{}) error {
	return json.NewDecoder(responseBody).Decode(target)
}

func getEnv(key string, defaultVal string) string {
	if env, ok := os.LookupEnv(key); ok {
		return env
	}
	return defaultVal
}

/*
##########################################
########## Helper functions End ##########
##########################################
*/

/*
##########################################
######## Send Request to RGW Ceph ########
##########################################
*/

func (collector *S3Collector) updateUserLimitStats(uid string) (UserStats, error) {
	// Generate the string to sign
	signature := generateSignature("GET", "/admin/user", collector.cephSecretKey)

	// Send the request
	url := fmt.Sprintf("%s/admin/user?quota&uid=%s&quota-type=user", collector.cephGatewayURL, uid)
	req, err := createRequest(url, collector.cephAccessKey, signature)
	if err != nil {
		return UserStats{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return UserStats{}, err
	}
	defer res.Body.Close()

	// Read the response
	var users UserStats
	err = decodeResponse(res.Body, &users)
	if err != nil {
		return UserStats{}, err
	}

	collector.userStats = users

	return users, nil
}

func (collector *S3Collector) updateBucketStatsMap(buckets []BucketStats) {
	collector.bucketStats = make(map[string]BucketStats)
	for _, bucket := range buckets {
		collector.bucketStats[bucket.Name] = bucket
	}
}

func (collector *S3Collector) updateBucketStats() error {
	// Generate the string to sign
	signature := generateSignature("GET", "/admin/bucket", collector.cephSecretKey)

	// Send the request
	client := &http.Client{}
	url := fmt.Sprintf("%s/admin/bucket?stats", collector.cephGatewayURL)
	req, err := createRequest(url, collector.cephAccessKey, signature)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Read the response
	var buckets []BucketStats
	err = decodeResponse(res.Body, &buckets)
	if err != nil {
		return err
	}

	// Update the bucket stats map
	collector.updateBucketStatsMap(buckets)

	return nil
}

/*
##########################################
##### Send Request to RGW Ceph END #######
##########################################
*/

/*
##########################################
#### Calculate And Publish Metrics #######
##########################################
*/

func GetAllUserLimits(collector *S3Collector, ch chan<- prometheus.Metric) {
	seenUsers := make(map[string]bool)
	for _, stats := range collector.bucketStats {
		if !seenUsers[stats.BucketOwner] {
			users, err := collector.updateUserLimitStats(string(stats.BucketOwner))
			if err != nil {
				return
			}
			ch <- prometheus.MustNewConstMetric(collector.userQuotaMaxSize, prometheus.GaugeValue, float64(users.MaxSize), stats.BucketOwner)
			ch <- prometheus.MustNewConstMetric(collector.userQuotaMaxObjects, prometheus.GaugeValue, float64(users.MaxObjects), stats.BucketOwner)
			seenUsers[stats.BucketOwner] = true
		}
	}
}

func CalculateBucketsTotalSizeMetric(collector *S3Collector, ch chan<- prometheus.Metric) {
	// // Calculate total size across all buckets
	var totalSize float64
	for _, stats := range collector.bucketStats {
		totalSize += float64(stats.Usage.SizeDetails.SizeActual)
	}

	ch <- prometheus.MustNewConstMetric(
		collector.totalUsageSizeActualMetric,
		prometheus.GaugeValue,
		totalSize,
	)
}

func CalculateBucketsSizesMetrics(collector *S3Collector, ch chan<- prometheus.Metric) {
	// Collect the metrics
	for bucketName, stats := range collector.bucketStats {
		ch <- prometheus.MustNewConstMetric(collector.sizeActualMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.SizeActual), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.sizeUtilizedMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.SizeUtilized), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.numObjectsMetric, prometheus.GaugeValue, float64(stats.Usage.SizeDetails.NumObjects), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.bucketNumShards, prometheus.GaugeValue, float64(stats.NumShards), bucketName)
	}
}

func PerUserUsageMetrics(collector *S3Collector, ch chan<- prometheus.Metric) {
	uniqueOwners := make(map[string]*float64, len(collector.bucketStats))

	for _, stats := range collector.bucketStats {
		owner := stats.BucketOwner
		totalSizePtr, exists := uniqueOwners[owner]
		if !exists {
			totalSize := float64(0)
			totalSizePtr = &totalSize
			uniqueOwners[owner] = totalSizePtr
		}
		*totalSizePtr += float64(stats.Usage.SizeDetails.SizeActual)
	}

	for owner, totalSizePtr := range uniqueOwners {
		ch <- prometheus.MustNewConstMetric(collector.perUserUsageMetric, prometheus.GaugeValue, *totalSizePtr, owner)
	}
}

func ExposeBucketQuotaMetrics(collector *S3Collector, ch chan<- prometheus.Metric) {
	for bucketName, stats := range collector.bucketStats {
		ch <- prometheus.MustNewConstMetric(collector.bucketQuotaMaxSize, prometheus.GaugeValue, float64(stats.BucketQuota.MaxSize), bucketName)
		ch <- prometheus.MustNewConstMetric(collector.bucketQuotaMaxObjects, prometheus.GaugeValue, float64(stats.BucketQuota.MaxObjects), bucketName)
	}
}

/*
##########################################
### Calculate And Publish Metrics END ####
##########################################
*/

func newS3Collector(cephAccessKey, cephSecretKey, cephGatewayURL string) *S3Collector {
	return &S3Collector{
		sizeActualMetric: prometheus.NewDesc("ceph_rgw_bucket_actual_size",
			"s3 bucket size",
			[]string{"name"}, nil,
		),
		totalUsageSizeActualMetric: prometheus.NewDesc("ceph_rgw_bucket_total_usage_size",
			"s3 total usage all buckets size",
			nil, nil,
		),
		sizeUtilizedMetric: prometheus.NewDesc("ceph_rgw_bucket_utilized_size",
			"s3 bucket utilized size",
			[]string{"name"}, nil,
		),
		numObjectsMetric: prometheus.NewDesc("ceph_rgw_bucket_num_objects",
			"s3 bucket number of objects",
			[]string{"name"}, nil,
		),
		perUserUsageMetric: prometheus.NewDesc("ceph_rgw_user_usage_size",
			"size of each user",
			[]string{"name"}, nil,
		),
		bucketQuotaMaxSize: prometheus.NewDesc("ceph_rgw_bucket_quota_max_size",
			"bucket quota max size",
			[]string{"name"}, nil,
		),
		bucketQuotaMaxObjects: prometheus.NewDesc("ceph_rgw_bucket_quota_max_objects",
			"bucket quota max objects",
			[]string{"name"}, nil,
		),
		bucketNumShards: prometheus.NewDesc("ceph_rgw_bucket_num_shards",
			"bucket number of shards",
			[]string{"name"}, nil,
		),
		userQuotaMaxSize: prometheus.NewDesc("ceph_rgw_user_quota_max_size",
			"User limit size",
			[]string{"user"}, nil,
		),
		userQuotaMaxObjects: prometheus.NewDesc("ceph_rgw_user_quota_max_objects",
			"User max number of objects",
			[]string{"user"}, nil,
		),
		cephAccessKey:  cephAccessKey,
		cephSecretKey:  cephSecretKey,
		cephGatewayURL: cephGatewayURL,
		bucketStats:    make(map[string]BucketStats),
	}
}

func (collector *S3Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.sizeActualMetric
	ch <- collector.totalUsageSizeActualMetric
	ch <- collector.numObjectsMetric
	ch <- collector.numObjectsMetric
	ch <- collector.perUserUsageMetric
	ch <- collector.bucketQuotaMaxObjects
	ch <- collector.bucketQuotaMaxSize
	ch <- collector.bucketNumShards
	ch <- collector.userQuotaMaxSize
	ch <- collector.userQuotaMaxObjects
}

func (collector *S3Collector) Collect(ch chan<- prometheus.Metric) {

	err := collector.updateBucketStats()
	if err != nil {
		log.Printf("Error updating bucket stats: %s", err)
		return
	}

	GetAllUserLimits(collector, ch)
	PerUserUsageMetrics(collector, ch)
	CalculateBucketsSizesMetrics(collector, ch)
	CalculateBucketsTotalSizeMetric(collector, ch)
	ExposeBucketQuotaMetrics(collector, ch)
}

func init() {
	prometheus.MustRegister(version.NewCollector(program))
}

func main() {
	var (
		printVersion      = flag.Bool("version", false, "Print version information.")
		listenAddress     = flag.String("web.listen-address", getEnv("LISTEN_ADDRESS", ":9290"), "Address to listen on for web interface and telemetry.")
		metricsPath       = flag.String("web.telemetry-path", getEnv("METRIC_PATH", "/metrics"), "Path under which to expose metrics.")
		GolangMetrics     = flag.Bool("web.golang-metrics", false, "Disable/enable default golang metrics 'true/false'")
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

	promReg := prometheus.NewRegistry()

	prometheus.MustRegister(s3)
	promReg.MustRegister(s3)

	var handler http.Handler

	if !*GolangMetrics {
		handler = promhttp.HandlerFor(
			promReg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: *GolangMetrics,
			},
		)
	} else {
		handler = promhttp.Handler()
	}

	fmt.Println("GolangMetrics", *GolangMetrics)

	http.Handle(*metricsPath, handler)
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
