//go:build !js

package ipevaluation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestSuppliedArchives verifies the ignored GeoLite archives load and answer lookups in memory.
func TestSuppliedArchives(t *testing.T) {
	var (
		directory string = filepath.Join("..", "..", "data")
		evaluator *Evaluator
		result    Result
		err       error
	)

	if _, err = os.Stat(directory); errors.Is(err, os.ErrNotExist) {
		t.Skip("local GeoLite archives are not available")
	} else if err != nil {
		t.Fatalf("inspect GeoLite archive directory: %v", err)
	}

	if evaluator, err = Open(directory); err != nil {
		t.Fatalf("open GeoLite archives: %v", err)
	}
	defer evaluator.Close()

	if result, err = evaluator.Evaluate("1.1.1.1"); err != nil {
		t.Fatalf("evaluate public IP: %v", err)
	}

	if result.ASN == 0 || result.ASNOrganization == "" || result.CountryCode == "" {
		t.Fatalf("lookup returned incomplete network data: %+v", result)
	}
}
