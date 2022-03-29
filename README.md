# grafana-local-sync

Docker image containing a go app that syncs dashboards stored in a directory to a local grafana instance. This allows developers to edit the dashboard using the grafana web UI and have their changes show up on their local disk.

## Sample Usage

```sh
# Assuming you have a local grafana running on port 3000 using something like:
# docker run --rm -d -p 3000:3000 grafana/grafana:latest

docker run --rm -it --mount type=bind,source=${LOCAL_DASHBOARD_DIRECTORY},target=${GRAFANA_DASHBOARD_DIRECTORY}/LocalDev --network="host" mintel/grafana-local-sync:latest -user admin -pass ${GRAFANA_ADMIN_PASSWORD} -dir ${GRAFANA_DASHBOARD_DIRECTORY}
```
