# S3 CEPH EXPORTER

A Prometheus exporter for monitoring s3 in ceph

```bash
./s3_ceph_exporter -radosgw.server "http://localhost:9000" \
                   -radosgw.access-secret "secret" \
                   -radosgw.access-key "key" \
                   -web.golang-metrics=true
```

## Run with Docker
You can execute the exporter using the Docker image.

```bash
docker pull sepehrimanian/s3-ceph-exporter
docker run -p 9290:9290 sepehrimanian/s3-ceph-exporter -radosgw.server "http://localhost:9000" \
                                                       -radosgw.access-secret "secret" \
                                                       -radosgw.access-key "key" \
                                                       -web.golang-metrics=true
```

The same result can be achieved with Enviroment variables.
* **LISTEN_ADDRESS**: is the exporter address, as the option *web.listen-address*
* **METRIC_PATH**: the telemetry path. It corresponds to *web.telemetry-path*
* **CEPH_URL**: the URL of the CEPH rados gateway server, as *radosgw.server*
* **CEPH_ACCESS_KEY**: the CEPH rados gateway access key (*radosgw.access-key*)
* **CEPH_ACCESS_SECRET**: the CEPH rados gateway access secret (*radosgw.access-secret*)


```bash
docker run \
       -p 9290:9290 \
       -e "CEPH_URL=http://localhost:9000" \
       -e "CEPH_ACCESS_KEY=key" \
       -e "CEPH_ACCESS_SECRET=secret" \
       sepehrimanian/s3-ceph-exporter
```

## Make
```
build: Go build
run: run go s3-ceph-exporter app
docker-build: docker build
docker-run: run docker container
compile: compiling for every OS and Platform
```

## Usage of `s3_ceph_exporter`

```bash
./s3_ceph_exporter --help
```

| Option                    | Default             | Description
| ------------------------- | ------------------- | -----------------
| -h, --help                | -                   | Displays usage.
| --version                 | -                   | Prints version information
| -web.listen-address       | `:9290`             | The address to listen on for HTTP requests.
| -web.telemetry-path       | `/metrics`          | URL Endpoint for metrics
| -web.golang-metrics       | `false`             | Enable default golang metrics.
| -radosgw.server           | `http://localhost:9000` | Ceph rados gateway url
| -radosgw.access-key       | -                  | Ceph rados access key
| -radosgw.access-secret    | -                  | Ceph rados access secret

## Metrics in prometheus
| Name          		            | Description     |
|-----------------------------| -------- |
| ceph_rgw_s3_actual_size			  | size of each s3 bucket.    |
| ceph_rgw_s3_total_usage_size	    | total usage all s3 buckets size.    |
| ceph_rgw_s3_utilized_size	       | utilized size each s3 bucket.    |
| ceph_rgw_s3_num_objects	         | number of objects each s3 bucket.    |
| ceph_rgw_s3_per_user_usage_size	 | size of each user in ceph.    |
| ceph_rgw_s3_quota_max_size	      | quota max size.    |
| ceph_rgw_s3_quota_max_objects	   | quota max objects.    |
| ceph_rgw_s3_num_shards	          | number of shards each s3 bucket.    |