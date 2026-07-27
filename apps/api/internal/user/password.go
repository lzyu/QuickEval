package user

import "golang.org/x/crypto/bcrypt"

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) BcryptHasher {
	if cost == 0 {
		cost = 12
	}
	return BcryptHasher{cost: cost}
}

func (hasher BcryptHasher) Hash(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), hasher.cost)
	return string(value), err
}

func (hasher BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
