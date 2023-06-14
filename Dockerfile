FROM golang:1.20.5-alpine3.18 AS build-env
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o s3-ceph-exporter .

FROM alpine:3.18.0
WORKDIR /app
COPY --from=build-env /app/s3-ceph-exporter .
EXPOSE 9290
CMD ["./s3-ceph-exporter"]