package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword genera el hash bcrypt de una contrasena en texto plano.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compara una contrasena en texto plano contra su hash bcrypt.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
