package notifications

import "testing"

func TestParseLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "default", value: "", want: defaultLimit},
		{name: "minimum", value: "1", want: 1},
		{name: "maximum", value: "100", want: 100},
		{name: "whitespace", value: " 25 ", want: 25},
		{name: "zero", value: "0", wantErr: true},
		{name: "too large", value: "101", wantErr: true},
		{name: "not numeric", value: "many", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseLimit(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseLimit(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
			if !test.wantErr && got != test.want {
				t.Fatalf("parseLimit(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}
