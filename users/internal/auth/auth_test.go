package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "s3cret-password" {
		t.Fatal("el hash no debe ser igual al texto plano")
	}
	if !CheckPassword(hash, "s3cret-password") {
		t.Fatal("CheckPassword deberia aceptar la contrasena correcta")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("CheckPassword no debe aceptar una contrasena incorrecta")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewTokenManager("test-secret", time.Minute, time.Hour)
	token, exp, err := m.GenerateAccess("user-123", "alice")
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}
	if !exp.After(time.Now()) {
		t.Fatal("la expiracion del access token debe ser futura")
	}
	claims, err := m.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if claims.Subject != "user-123" || claims.Username != "alice" {
		t.Fatalf("claims inesperados: %+v", claims)
	}
}

func TestParseAccessRejectsTampered(t *testing.T) {
	m := NewTokenManager("test-secret", time.Minute, time.Hour)
	token, _, _ := m.GenerateAccess("user-123", "alice")
	if _, err := m.ParseAccess(token + "x"); err == nil {
		t.Fatal("un token alterado debe ser rechazado")
	}
	other := NewTokenManager("other-secret", time.Minute, time.Hour)
	if _, err := other.ParseAccess(token); err == nil {
		t.Fatal("un token firmado con otra clave debe ser rechazado")
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	m := NewTokenManager("test-secret", time.Minute, time.Hour)
	token, _, err := m.GenerateRefresh("user-123")
	if err != nil {
		t.Fatalf("GenerateRefresh: %v", err)
	}
	userID, err := m.ParseRefresh(token)
	if err != nil {
		t.Fatalf("ParseRefresh: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("subject inesperado: %s", userID)
	}
}

func TestHashTokenIsStable(t *testing.T) {
	if HashToken("abc") != HashToken("abc") {
		t.Fatal("HashToken debe ser determinista")
	}
	if HashToken("abc") == HashToken("abd") {
		t.Fatal("HashToken debe distinguir entradas distintas")
	}
}
