//go:build !webui

package webui

import "testing"

func TestAssetsDisabledWithoutBuildTag(t *testing.T) {
	assets, err := Assets()
	if err != nil {
		t.Fatalf("Assets() error = %v", err)
	}
	if assets != nil {
		t.Fatalf("Assets() = %#v, want nil without webui tag", assets)
	}
}
