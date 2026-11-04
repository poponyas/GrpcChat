package auth

import "testing"

func TestPasswordHashes(t *testing.T) {
	a, err := passwordHash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	b, err := passwordHash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("passwords must have random salts")
	}
	if !checkPassword("correct horse battery", a) {
		t.Fatal("valid password rejected")
	}
	if checkPassword("wrong password", a) || checkPassword("anything", "broken") {
		t.Fatal("invalid password accepted")
	}
}
