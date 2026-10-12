package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestStoreIntegration requiere una base de datos real. Se salta si no se
// define TEST_DATABASE_URL. Ejemplo:
//
//	TEST_DATABASE_URL=postgres://casino:casino@localhost:5432/users_db?sslmode=disable go test ./...
func TestStoreIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no definido; se omite el test de integracion")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	email := uuid.NewString() + "@example.com"
	u := &User{
		ID:           uuid.NewString(),
		Username:     "user_" + uuid.NewString()[:8],
		Email:        email,
		PasswordHash: "hash",
		Status:       "active",
	}
	if err := st.Users.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Users.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("id inesperado: %s != %s", got.ID, u.ID)
	}

	dup := &User{ID: uuid.NewString(), Username: u.Username, Email: email, PasswordHash: "hash", Status: "active"}
	if err := st.Users.Create(ctx, dup); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("se esperaba ErrEmailTaken, se obtuvo %v", err)
	}
}
