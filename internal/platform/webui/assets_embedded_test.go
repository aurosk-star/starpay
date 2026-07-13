//go:build webui

package webui

import (
	"io/fs"
	"testing"
)

func TestAssetsEmbeddedWithBuildTag(t *testing.T) {
	assets, err := Assets()
	if err != nil {
		t.Fatalf("Assets() error = %v", err)
	}
	content, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("embedded index.html is empty")
	}
}
