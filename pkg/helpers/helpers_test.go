package helpers

import "testing"

func TestValidateURL(t *testing.T) {

	err := ValidateURL("http://")
	t.Errorf("unexpected error: %v", err)
}
