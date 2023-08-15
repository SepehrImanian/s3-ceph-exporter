package collector

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type UserStatsCollector struct {
	userQuotaMaxSize    *prometheus.Desc
	userQuotaMaxObjects *prometheus.Desc
	userStats           UserStats
}

type UserStats struct {
	MaxSize    float64 `json:"max_size"`
	MaxObjects float64 `json:"max_objects"`
}

func (collector *Collector) updateUserLimitStats(uid string) (UserStats, error) {
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

	var c *UserStatsCollector
	c.userStats = users

	return users, nil
}
