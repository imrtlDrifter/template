CREATE INDEX CONCURRENTLY trips_stale_position_idx
    ON trips (COALESCE(last_position_at, started_at), id)
    WHERE status = 'active';
