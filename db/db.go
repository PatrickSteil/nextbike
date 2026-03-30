package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

type DB struct {
	sql    *sql.DB
	upsert *sql.Stmt
	byUID  *sql.Stmt
}

func Open(path string) (*DB, error) {
	// _loc=UTC ensures timestamps are interpreted as UTC when read back
	sqlDB, err := sql.Open("sqlite3", path+"?_journal=WAL&_busy_timeout=5000&_loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	upsert, err := sqlDB.Prepare(`
		INSERT INTO stations (uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(uid) DO UPDATE SET
			name                    = excluded.name,
			city_uid                = excluded.city_uid,
			city_name               = excluded.city_name,
			lat                     = excluded.lat,
			lng                     = excluded.lng,
			bikes_available_to_rent = excluded.bikes_available_to_rent,
			updated_at              = excluded.updated_at
	`)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("prepare upsert: %w", err)
	}

	byUID, err := sqlDB.Prepare(`
		SELECT uid, name, city_uid, city_name, lat, lng, bikes_available_to_rent, updated_at
		FROM stations WHERE uid = ?
	`)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("prepare byUID: %w", err)
	}

	return &DB{sql: sqlDB, upsert: upsert, byUID: byUID}, nil
}

func (d *DB) Close() error {
	d.upsert.Close()
	d.byUID.Close()
	return d.sql.Close()
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

func (d *DB) Begin(ctx context.Context) (*sql.Tx, error) {
	return d.sql.BeginTx(ctx, nil)
}

func (d *DB) ByUID(ctx context.Context, uid int) (*Station, error) {
	row := d.byUID.QueryRowContext(ctx, uid)
	var s Station
	err := row.Scan(
		&s.UID, &s.Name, &s.CityUID, &s.CityName,
		&s.Lat, &s.Lng, &s.BikesAvailableToRent,
		&s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan station: %w", err)
	}
	return &s, nil
}

func migrate(sqlDB *sql.DB) error {
	_, err := sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS stations (
			uid                     INTEGER PRIMARY KEY,
			name                    TEXT    NOT NULL,
			city_uid                INTEGER NOT NULL,
			city_name               TEXT    NOT NULL,
			lat                     REAL    NOT NULL,
			lng                     REAL    NOT NULL,
			bikes_available_to_rent INTEGER NOT NULL DEFAULT 0,
			updated_at              TIMESTAMP NOT NULL
		)
	`)
	return err
}
