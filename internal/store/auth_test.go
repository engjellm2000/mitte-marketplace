package store

import (
	"context"
	"testing"
)

func TestCreateUserRejectsUnsupportedCustomerType(t *testing.T) {
	if _, err := (&Store{}).CreateUser(context.Background(), "person@example.test", "hash", "admin"); err == nil {
		t.Fatal("CreateUser() accepted an unsupported customer type")
	}
}
