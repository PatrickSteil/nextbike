# nextbike

Polls the [Nextbike Maps API](https://maps.nextbike.net) every 60 seconds and stores the latest bike availability per station in a local SQLite database. Exposes a small HTTP API for querying stations and cities.

Currently hardcoded to only fetch Germany stations (see `cmd/nextbike/main.go`).

## Setup
```bash
git clone https://github.com/PatrickSteil/nextbike
cd nextbike
go mod tidy
```

## Usage

| Command | Description |
|---|---|
| `make build` | Compile the binary |
| `make run` | Build and run |
| `make tidy` | Run `go mod tidy` |
| `make clean` | Remove the binary |
| `make reset` | Remove binary and database |
| `make db` | Print top 20 stations by available bikes |

On first launch the poller fetches immediately, then every 60 seconds. Every 5 minutes it removes stations that have not been updated in the last 10 minutes. The database file `nextbike.db` is created in the working directory.

## HTTP API
```
GET /cities
GET /cities/{uid}/stations
GET /stations
GET /stations/{uid}
GET /stations/nearby?lat=&lon=&radius=
```

All endpoints return JSON. The `uid` path parameter is the Nextbike place or city UID. **404** is returned if the UID has not been seen yet (wait for the first poll).

### Examples
```bash
# Single station
curl http://localhost:8080/stations/19153166
```
```json
{
  "UID": 19153166,
  "Name": "S-Bahnhof Kirchheim/Rohrbach",
  "CityUID": 462,
  "CityName": "Heidelberg",
  "Lat": 49.378751,
  "Lng": 8.67634,
  "BikesAvailableToRent": 6,
  "UpdatedAt": "2026-03-30T10:00:00Z"
}
```
```bash
# Stations within 1 km of a coordinate
curl "http://localhost:8080/stations/nearby?lat=49.3988&lon=8.6724&radius=1"
```

## Database

The database is a plain SQLite file using WAL mode. Query it directly:
```bash
sqlite3 nextbike.db "SELECT uid, name, bikes_available_to_rent, updated_at FROM stations ORDER BY bikes_available_to_rent DESC LIMIT 20;"
```

### Schema
```sql
CREATE TABLE IF NOT EXISTS stations (
    uid                     INTEGER   PRIMARY KEY,
    name                    TEXT      NOT NULL,
    city_uid                INTEGER   NOT NULL,
    city_name               TEXT      NOT NULL,
    lat                     REAL      NOT NULL,
    lng                     REAL      NOT NULL,
    bikes_available_to_rent INTEGER   NOT NULL DEFAULT 0,
    updated_at              TIMESTAMP NOT NULL
);

-- R*Tree index for spatial queries
CREATE VIRTUAL TABLE IF NOT EXISTS stations_rtree USING rtree(
    id,
    min_lat, max_lat,
    min_lng, max_lng
);
```

Spatial queries (`/stations/nearby`) use the R\*Tree index for an efficient bounding-box pre-filter followed by an exact Haversine check.

If you change the schema, delete `nextbike.db` before restarting (`make reset`).
