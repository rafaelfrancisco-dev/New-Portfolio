package db

import (
	"context"
	"database/sql"
	"time"
)

// ReserveLogin is a single SQLite write statement: concurrent requests cannot
// observe an available budget without reserving it. No transaction spans hashing.
func ReserveLogin(ctx context.Context, d *sql.DB, client string) (bool, error) {
	result, err := d.ExecContext(ctx, `INSERT INTO login_attempts(ip_address)
 SELECT ? WHERE (SELECT count(*) FROM login_attempts WHERE attempted_at > datetime('now','-15 minutes')) < 30
 AND (SELECT count(*) FROM login_attempts WHERE ip_address=? AND attempted_at > datetime('now','-15 minutes')) < 5`, client, client)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func RevokeSessions(ctx context.Context, d *sql.DB) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{`UPDATE admin_security SET generation=generation+1 WHERE id=1`, `DELETE FROM admin_sessions`, `DELETE FROM flickr_oauth_requests`} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func PruneSecurity(ctx context.Context, d *sql.DB) error {
	for _, statement := range []string{`DELETE FROM admin_sessions WHERE julianday(expires_at)<=julianday('now')`, `DELETE FROM flickr_oauth_requests WHERE julianday(expires_at)<=julianday('now')`, `DELETE FROM login_attempts WHERE attempted_at<=datetime('now','-15 minutes')`} {
		if _, err := d.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func CreateBoundSession(ctx context.Context, d *sql.DB, token, csrf, fingerprint string, generation int64, expires time.Time) (bool, error) {
	result, err := d.ExecContext(ctx, `INSERT INTO admin_sessions(token_hash,csrf_token,expires_at,credential_fingerprint,generation) SELECT ?,?,?,?,generation FROM admin_security WHERE id=1 AND generation=?`, token, csrf, expires, fingerprint, generation)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
