package collector

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type BucketStatsCollector struct {
	sizeActualMetric           *prometheus.Desc
	totalUsageSizeActualMetric *prometheus.Desc
	sizeUtilizedMetric         *prometheus.Desc
	numObjectsMetric           *prometheus.Desc
	perUserUsageMetric         *prometheus.Desc
	bucketQuotaMaxSize         *prometheus.Desc
	bucketQuotaMaxObjects      *prometheus.Desc
	bucketNumShards            *prometheus.Desc
	bucketStats                map[string]BucketStats
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

func (collector *Collector) updateBucketStatsMap(buckets []BucketStats) {
	var c *BucketStatsCollector
	c.bucketStats = make(map[string]BucketStats)
	for _, bucket := range buckets {
		c.bucketStats[bucket.Name] = bucket
	}
}

func (collector *Collector) updateBucketStats() error {
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
