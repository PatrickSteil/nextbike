package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	_ "modernc.org/sqlite"
)

const earthRadiusKm = 6371.0

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

type Station struct {
	UID                  int
	Name                 string
	CityUID              int
	CityName             string
	Lat, Lng             float64
	BikesAvailableToRent int
	UpdatedAt            time.Time
}

type City struct {
	UID          int
	Name         string
	StationCount int
	TotalBikes   int
}

type DB struct {
	sql         *sql.DB
	upsert      *sql.Stmt
	byUID       *sql.Stmt
	allStations *sql.Stmt
	updateLive  *sql.Stmt
	byCity      *sql.Stmt
	allCities   *sql.Stmt
	withinBox   *sql.Stmt
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal=WAL&_busy_timeout=5000&_loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(`PRAGMA auto_vacuum = FULL;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set pragma: %w", err)
	}

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	stmts := map[string]**sql.Stmt{}
	queries := map[string]string{
		"upsert": `
			INSERT INTO stations (uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(uid) DO UPDATE SET
				name                    = excluded.name,
				city_uid                = excluded.city_uid,
				city_name               = excluded.city_name,
				lat                     = excluded.lat,
				lng                     = excluded.lng,
				bikes_available_to_rent = excluded.bikes_available_to_rent,
				updated_at              = excluded.updated_at`,
		"byUID": `
			SELECT uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at
			FROM stations WHERE uid = ?`,
		"allStations": `
			SELECT uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at
			FROM stations ORDER BY name`,
		"byCity": `
			SELECT uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at
			FROM stations WHERE city_uid = ? ORDER BY name`,
		"allCities": `
			SELECT city_uid, city_name, COUNT(*) AS station_count, SUM(bikes_available_to_rent) AS total_bikes
			FROM stations GROUP BY city_uid, city_name ORDER BY city_name`,
		"updateLive": `
    INSERT INTO stations (uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(uid) DO UPDATE SET
        bikes_available_to_rent = excluded.bikes_available_to_rent,
        updated_at              = excluded.updated_at`,

		"withinBox": `
			SELECT s.uid, s.name, s.city_uid, s.city_name, s.lat, s.lng, s.bikes_available_to_rent, s.updated_at
			FROM stations s
			JOIN stations_rtree r ON s.uid = r.id
			WHERE r.min_lat >= ? AND r.max_lat <= ?
			  AND r.min_lng >= ? AND r.max_lng <= ?`,
	}

	d := &DB{sql: sqlDB}
	stmts["upsert"] = &d.upsert
	stmts["byUID"] = &d.byUID
	stmts["allStations"] = &d.allStations
	stmts["updateLive"] = &d.updateLive
	stmts["byCity"] = &d.byCity
	stmts["allCities"] = &d.allCities
	stmts["withinBox"] = &d.withinBox

	for name, ptr := range stmts {
		*ptr, err = sqlDB.Prepare(queries[name])
		if err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("prepare %s: %w", name, err)
		}
	}

	return d, nil
}

func (d *DB) Close() error {
	d.upsert.Close()
	d.byUID.Close()
	d.allStations.Close()
	d.updateLive.Close()
	d.byCity.Close()
	d.allCities.Close()
	d.withinBox.Close()
	return d.sql.Close()
}

func (d *DB) Begin(ctx context.Context) (*sql.Tx, error) {
	return d.sql.BeginTx(ctx, nil)
}

func (d *DB) Upsert(ctx context.Context, tx *sql.Tx, s Station) error {
	stmt := tx.StmtContext(ctx, d.upsert)
	_, err := stmt.ExecContext(ctx,
		s.UID, s.Name, s.CityUID, s.CityName,
		s.Lat, s.Lng, s.BikesAvailableToRent,
		s.UpdatedAt.UTC(),
	)
	return err
}

func (d *DB) ByUID(ctx context.Context, uid int) (*Station, error) {
	return scanStation(d.byUID.QueryRowContext(ctx, uid))
}

func (d *DB) AllStations(ctx context.Context) ([]Station, error) {
	rows, err := d.allStations.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("all stations: %w", err)
	}
	return collectStations(rows)
}

func (d *DB) StationsByCity(ctx context.Context, cityUID int) ([]Station, error) {
	rows, err := d.byCity.QueryContext(ctx, cityUID)
	if err != nil {
		return nil, fmt.Errorf("stations by city: %w", err)
	}
	return collectStations(rows)
}

func (d *DB) AllCities(ctx context.Context) ([]City, error) {
	rows, err := d.allCities.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("all cities: %w", err)
	}
	defer rows.Close()

	var cities []City
	for rows.Next() {
		var c City
		if err := rows.Scan(&c.UID, &c.Name, &c.StationCount, &c.TotalBikes); err != nil {
			return nil, fmt.Errorf("scan city: %w", err)
		}
		cities = append(cities, c)
	}
	return cities, rows.Err()
}

func scanStation(row *sql.Row) (*Station, error) {
	var s Station
	err := row.Scan(
		&s.UID, &s.Name, &s.CityUID, &s.CityName,
		&s.Lat, &s.Lng, &s.BikesAvailableToRent, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan station: %w", err)
	}
	return &s, nil
}

func collectStations(rows *sql.Rows) ([]Station, error) {
	defer rows.Close()
	var stations []Station
	for rows.Next() {
		var s Station
		if err := rows.Scan(
			&s.UID, &s.Name, &s.CityUID, &s.CityName,
			&s.Lat, &s.Lng, &s.BikesAvailableToRent, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan station: %w", err)
		}
		stations = append(stations, s)
	}
	return stations, rows.Err()
}

func (d *DB) StationsWithinRadius(ctx context.Context, lat, lng float64, radiusKm float64) ([]Station, error) {
	latDelta := (radiusKm / earthRadiusKm) * (180.0 / math.Pi)

	lngDelta := latDelta / math.Cos(lat*math.Pi/180.0)

	minLat := lat - latDelta
	maxLat := lat + latDelta
	minLng := lng - lngDelta
	maxLng := lng + lngDelta

	rows, err := d.withinBox.QueryContext(ctx, minLat, maxLat, minLng, maxLng)
	if err != nil {
		return nil, fmt.Errorf("query within box: %w", err)
	}
	defer rows.Close()

	var stations []Station
	for rows.Next() {
		var s Station
		if err := rows.Scan(
			&s.UID, &s.Name, &s.CityUID, &s.CityName,
			&s.Lat, &s.Lng, &s.BikesAvailableToRent, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan station: %w", err)
		}

		if haversineDistance(lat, lng, s.Lat, s.Lng) <= radiusKm {
			stations = append(stations, s)
		}
	}

	return stations, rows.Err()
}

func (d *DB) UpsertLive(ctx context.Context, stations []Station) error {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := tx.StmtContext(ctx, d.updateLive)
	for _, s := range stations {
		_, err := stmt.ExecContext(ctx,
			s.UID, s.Name, s.CityUID, s.CityName,
			s.Lat, s.Lng, s.BikesAvailableToRent,
			s.UpdatedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("upsert station %d: %w", s.UID, err)
		}
	}
	return tx.Commit()
}

func migrate(sqlDB *sql.DB) error {
	_, err := sqlDB.Exec(`
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

		-- Create the R*Tree index table
		CREATE VIRTUAL TABLE IF NOT EXISTS stations_rtree USING rtree(
			id,
			min_lat, max_lat,
			min_lng, max_lng
		);

		-- Trigger to add a node to the R*Tree on insert
		CREATE TRIGGER IF NOT EXISTS stations_ai AFTER INSERT ON stations BEGIN
			INSERT INTO stations_rtree(id, min_lat, max_lat, min_lng, max_lng)
			VALUES(new.uid, new.lat, new.lat, new.lng, new.lng);
		END;

		-- Trigger to update the R*Tree on location update
		CREATE TRIGGER IF NOT EXISTS stations_au AFTER UPDATE ON stations BEGIN
			UPDATE stations_rtree SET
				min_lat = new.lat, max_lat = new.lat,
				min_lng = new.lng, max_lng = new.lng
			WHERE id = old.uid;
		END;

		-- Trigger to remove from R*Tree on delete
		CREATE TRIGGER IF NOT EXISTS stations_ad AFTER DELETE ON stations BEGIN
			DELETE FROM stations_rtree WHERE id = old.uid;
		END;

		INSERT INTO stations_rtree (id, min_lat, max_lat, min_lng, max_lng)
        SELECT uid, lat, lat, lng, lng FROM stations
        WHERE uid NOT IN (SELECT id FROM stations_rtree);
	`)
	return err
}
