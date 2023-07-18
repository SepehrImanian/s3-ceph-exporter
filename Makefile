radosgw_server="http://localhost:9000"
radosgw_access_secret="secret"
radosgw_access_key="key"
golang_metrics=true

build:
	go build -o s3-ceph-exporter s3-ceph-exporter.go

run:
	go run main.go -radosgw.server $(radosgw_server) \
				   -radosgw.access-secret $(radosgw_access_secret) \
				   -radosgw.access-key $(radosgw_access_key) \
				   -web.golang-metrics=$(golang_metrics)

docker-build:
	docker build -t s3-ceph-exporter .

docker-run:
	docker run -it --rm -p 9290:9290 s3-ceph-exporter

compile:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=arm go build -o bin/s3-ceph-exporter-arm main.go
	GOOS=linux GOARCH=arm64 go build -o bin/s3-ceph-exporter-arm64 main.go
	GOOS=freebsd GOARCH=386 go build -o bin/s3-ceph-exporter-freebsd-386 main.go