package output

import (
	"os"
	"path/filepath"
	"testing"
)

type node struct {
	Resource string `json:"resource"`
	Title    string `json:"title"`
	Links    []node `json:"links"`
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestWriteJSONContent(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name: "tree with ampersand and empty links",
			input: []node{{
				Resource: "https://a.com/",
				Title:    "A",
				Links: []node{{
					Resource: "https://a.com/b?x=1&y=2",
					Title:    "B",
					Links:    []node{},
				}},
			}},
			want: `[
  {
    "resource": "https://a.com/",
    "title": "A",
    "links": [
      {
        "resource": "https://a.com/b?x=1&y=2",
        "title": "B",
        "links": []
      }
    ]
  }
]
`,
		},
		{
			name:  "empty slice",
			input: []node{},
			want:  "[]\n",
		},
		{
			name:  "nil slice",
			input: []node(nil),
			want:  "null\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")

			if err := WriteJSON(path, tc.input); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := readFile(t, path); got != tc.want {
				t.Errorf("content:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestWriteJSONNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")

	if err := WriteJSON(path, []node{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "result.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("files in dir = %v, want [result.json]", names)
	}
}

func TestWriteJSONMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "result.json")

	if err := WriteJSON(path, []node{}); err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestWriteJSONOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")

	if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteJSON(path, []node{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := readFile(t, path); got != "[]\n" {
		t.Errorf("content = %q, want %q", got, "[]\n")
	}
}
