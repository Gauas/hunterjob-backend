package config

import "testing"

func TestValidateAPIRequiresPort(t *testing.T) {
	if err := (Config{Port: " "}).ValidateAPI(); err == nil {
		t.Fatal("expected blank API_PORT to be rejected")
	}
	if err := (Config{Port: "8080"}).ValidateAPI(); err != nil {
		t.Fatal(err)
	}
}
