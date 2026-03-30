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
