package goeyes

import (
	"net"
	"testing"

	"github.com/hawkinsw/goeyes"
)

func TestHappyEyeballs_InvalidHost(t *testing.T) {
	_, err := goeyes.HappyEyeballs(nil, "google/com", 80)
	_, coercion_success := err.(goeyes.InvalidHostname)
	if !coercion_success {
		t.Fatalf("A invalid hostname error was expected but not received.")
	}
}

func TestHappyEyeballs_NoDNS(t *testing.T) {
	_, err := goeyes.HappyEyeballs(nil, "bad.example.com", 80)
	_, coercion_success := err.(*net.DNSError)
	if !coercion_success {
		t.Fatalf("A DNS error was expected but not received.")
	}
}

func TestHappyEyeballs_Success(t *testing.T) {
	goeyes.HappyEyeballs(nil, "google.com", 80)
}
