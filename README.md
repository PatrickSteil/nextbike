# nextbike

Polls the [Nextbike Maps API](https://maps.nextbike.net) every 60 seconds and stores the latest bike availability per station in a local SQLite database. Exposes a small HTTP API for querying stations by UID.

Currently hardcoded to only fetch the Germany stations (see `cmd/nextbike/main.go`).

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

On first launch the poller fetches immediately, then every 60 seconds. The database file `nextbike.db` is created in the working directory.

## HTTP API

```
GET /stations/{uid}
```

Returns the latest state of a station. The UID is the Nextbike place UID.

```bash
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

**404** if the station UID has not been seen yet (wait for the first poll).

## Database

The database is a plain SQLite file. Query it directly:

```bash
sqlite3 nextbike.db "SELECT uid, name, bikes_available_to_rent, updated_at FROM stations ORDER BY bikes_available_to_rent DESC LIMIT 20;"
```

If you change the schema, delete `nextbike.db` before restarting (`make reset`).
