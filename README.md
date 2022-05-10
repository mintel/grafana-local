# grafana-local-sync

Docker image containing a go app that syncs dashboards stored in a directory to a local grafana instance. This allows developers to edit the dashboard using the grafana web UI and have their changes show up on their local disk.

This is used in the [make grafana/develop command in Mintel's build-harness-extensions](https://github.com/mintel/build-harness-extensions/blob/main/modules/grafana/Makefile).

## Sample Usage

```sh
# You need to have a grafana instance running on port 3000 using something like:
docker run --rm -d -p 3000:3000 --name grafana_local grafana/grafana:latest

export LOCAL_DASHBOARD_DIRECTORY = # <relative path to directory containing JSON dashboard definitions>
export GRAFANA_ADMIN_PASSWORD = # <password you want to use to log in to local grafana, default is admin>

# grafana requires you to reset the admin password on your grafana instance before any dashboards can be loaded, either through the web UI or using:
docker exec -it grafana_local grafana-cli --homepath "/usr/share/grafana" admin reset-admin-password ${GRAFANA_ADMIN_PASSWORD}

docker run --rm -it --mount type=bind,source=${LOCAL_DASHBOARD_DIRECTORY},target=/app/dashboards/LocalDev --network="host" mintel/grafana-local-sync:latest -user admin -pass ${GRAFANA_ADMIN_PASSWORD} -dir /app/dashboards
```
