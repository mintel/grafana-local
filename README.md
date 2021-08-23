# Grafana Dashboard Local Dev

Testing a local dev setup for developing Grafana dashboards.

## Build

```sh
go build ./cmd/syncer
```

## Run

```sh
docker run --rm -it -p 3000:3000 grafana/grafana:latest

# Login into the instance and change the admin password.

./syncer -user admin -pass PASSWORD -dir ./dashboards
```

This will load the dashboards from on-disk to the Grafana container, and then - in a loop - fetch the dashboards from Grafana and write them to disk.
