package domain

import (
	"reflect"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "commas and whitespace", input: "Daily, Operations ,  AI", want: []string{"Daily", "Operations", "AI"}},
		{name: "normalizes duplicates", input: "#Daily; daily\nOperations", want: []string{"Daily", "Operations"}},
		{name: "empty", input: " , ; \n", want: []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseTags(test.input); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ParseTags(%q) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}
