package util

import (
	"reflect"
	"testing"
)

func TestFindBasesFromVendor(t *testing.T) {
	vendorPath := "../fixtures/vendor/"
	got := FindBasesFromVendor(vendorPath)
	want := []string{vendorPath + "katalog/aws/dashboard", vendorPath + "katalog/logging/curator", vendorPath + "katalog/logging/withyamlextention"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v\n", got, want)
	}
}
