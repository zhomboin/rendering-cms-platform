package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type DevAdminConfig struct {
	Enabled  bool
	Email    string
	Name     string
	Password string
}

type devAdminExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

func EnsureDevAdmin(ctx context.Context, exec devAdminExecutor, cfg DevAdminConfig) error {
	if !cfg.Enabled {
		return nil
	}

	email := strings.TrimSpace(strings.ToLower(cfg.Email))
	name := strings.TrimSpace(cfg.Name)
	password := strings.TrimSpace(cfg.Password)
	if email == "" || name == "" || password == "" {
		return errors.New("dev admin email, name and password are required")
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	_, err = exec.Exec(ctx, `
insert into users (
  email,
  name,
  password_hash,
  role
) values (
  $1,
  $2,
  $3,
  'admin'
)
on conflict (email) do update set
  name = excluded.name,
  password_hash = excluded.password_hash,
  role = excluded.role,
  updated_at = now()
`, email, name, passwordHash)
	return err
}
