-- One row per station. The poller UPSERTs on every tick,
-- so this table always reflects the latest observed state.
CREATE TABLE IF NOT EXISTS stations (
    uid         INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    city_uid    INTEGER NOT NULL,
    city_name   TEXT    NOT NULL,
    lat         REAL    NOT NULL,
    lng         REAL    NOT NULL,
    bikes       INTEGER NOT NULL DEFAULT 0,
    free_racks  INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT    NOT NULL  -- RFC3339 UTC
);

-- Fast lookup by UID is the primary key, so no extra index needed.
-- If you later want to filter by city:
-- CREATE INDEX IF NOT EXISTS idx_stations_city ON stations(city_uid);
