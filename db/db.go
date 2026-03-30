package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

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
	byCity      *sql.Stmt
	allCities   *sql.Stmt
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal=WAL&_busy_timeout=5000&_loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

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
	}

	d := &DB{sql: sqlDB}
	stmts["upsert"] = &d.upsert
	stmts["byUID"] = &d.byUID
	stmts["allStations"] = &d.allStations
	stmts["byCity"] = &d.byCity
	stmts["allCities"] = &d.allCities

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
	d.byCity.Close()
	d.allCities.Close()
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
		)
	`)
	return err
}
