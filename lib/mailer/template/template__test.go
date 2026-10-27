package template

import (
	"errors"
	"os"
	"testing"
	"testing/fstest"

	"github.com/biairmal/go-sdk/lib/errorz"
)

func TestRegistry_Render(t *testing.T) {
	reg, err := New(os.DirFS("testdata"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := map[string]string{"Name": "<Bia>", "Link": "https://app.test/verify#token=a b", "Code": "123456"}
	tests := []struct {
		name     string
		template string
		wantText string
		wantHTML string
		wantErr  error
	}{
		{
			name:     "text and html bodies; html escapes",
			template: "welcome",
			wantText: "Hello <Bia>,\n\nOpen https://app.test/verify#token=a b to continue.\n",
			wantHTML: "<p>Hello &lt;Bia&gt;,</p>\n<p><a href=\"https://app.test/verify#token=a%20b\">Continue</a></p>\n",
		},
		{name: "text body only", template: "code", wantText: "Your code is 123456.\n"},
		{name: "unknown name", template: "missing", wantErr: errorz.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, html, err := reg.Render(tt.template, data)
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("Render() error = %v, want %v", err, tt.wantErr)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
			if html != tt.wantHTML {
				t.Errorf("html = %q, want %q", html, tt.wantHTML)
			}
		})
	}
}

func TestNew_Errors(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
	}{
		{name: "text parse error", fsys: fstest.MapFS{"bad.txt": {Data: []byte("{{.Name")}}},
		{name: "html parse error", fsys: fstest.MapFS{"bad.html": {Data: []byte("{{if}}")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.fsys); err == nil {
				t.Fatal("New accepted a template that does not parse")
			}
		})
	}
}

func TestRegistry_RenderExecError(t *testing.T) {
	reg, err := New(fstest.MapFS{"x.txt": {Data: []byte("{{.Name.Missing}}")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := reg.Render("x", map[string]string{"Name": "a"}); err == nil {
		t.Fatal("Render succeeded on a failing template")
	}
}
