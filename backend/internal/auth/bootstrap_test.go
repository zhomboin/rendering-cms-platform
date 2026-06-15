package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type devAdminExecStub struct {
	called bool
	query  string
	args   []interface{}
}

func (s *devAdminExecStub) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	s.called = true
	s.query = sql
	s.args = arguments
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestEnsureDevAdminSkipsWhenDisabled(t *testing.T) {
	exec := &devAdminExecStub{}

	if err := EnsureDevAdmin(context.Background(), exec, DevAdminConfig{}); err != nil {
		t.Fatalf("EnsureDevAdmin() returned error: %v", err)
	}

	if exec.called {
		t.Fatal("Exec should not be called when bootstrap is disabled")
	}
}

func TestEnsureDevAdminUpsertsAdminWithHashedPassword(t *testing.T) {
	exec := &devAdminExecStub{}

	err := EnsureDevAdmin(context.Background(), exec, DevAdminConfig{
		Enabled:  true,
		Email:    " Admin@Rendering.Me ",
		Name:     " Dev Admin ",
		Password: "rendering_dev_password",
	})
	if err != nil {
		t.Fatalf("EnsureDevAdmin() returned error: %v", err)
	}

	if !exec.called {
		t.Fatal("Exec should be called when bootstrap is enabled")
	}
	if !strings.Contains(exec.query, "on conflict (email) do update") {
		t.Fatalf("query = %q, want upsert by email", exec.query)
	}
	if exec.args[0] != "admin@rendering.me" {
		t.Fatalf("email arg = %#v, want normalized email", exec.args[0])
	}
	if exec.args[1] != "Dev Admin" {
		t.Fatalf("name arg = %#v, want trimmed name", exec.args[1])
	}
	hash, ok := exec.args[2].(string)
	if !ok || !VerifyPassword(hash, "rendering_dev_password") {
		t.Fatalf("password hash arg should verify configured password")
	}
}

func TestEnsureDevAdminRejectsMissingRequiredFields(t *testing.T) {
	exec := &devAdminExecStub{}

	err := EnsureDevAdmin(context.Background(), exec, DevAdminConfig{
		Enabled: true,
		Email:   "admin@rendering.me",
		Name:    "Dev Admin",
	})
	if err == nil {
		t.Fatal("EnsureDevAdmin() returned nil error, want missing password error")
	}
	if exec.called {
		t.Fatal("Exec should not be called when config is invalid")
	}
}
