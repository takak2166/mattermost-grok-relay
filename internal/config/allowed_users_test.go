package config

import "testing"

func TestParseCommaSeparatedIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"single", "user-a", []string{"user-a"}},
		{"trim", " user-a , user-b ", []string{"user-a", "user-b"}},
		{"empty elements", "a,,b, ,c", []string{"a", "b", "c"}},
		{"empty string", "", nil},
		{"only commas", ", ,", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseCommaSeparatedIDs(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAllowedUserIDs(t *testing.T) {
	t.Parallel()
	set, err := AllowedUserIDs(func(key string) (string, bool) {
		if key == envAllowedUserIDs {
			return "11111111-1111-1111-1111-111111111111, 22222222-2222-2222-2222-222222222222", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if !set.Contains("11111111-1111-1111-1111-111111111111") {
		t.Fatal("expected first id")
	}
	if !set.Contains("22222222-2222-2222-2222-222222222222") {
		t.Fatal("expected second id")
	}
	if set.Contains("other") {
		t.Fatal("unexpected id")
	}
}

func TestAllowedUserIDsMissing(t *testing.T) {
	t.Parallel()
	_, err := AllowedUserIDs(func(string) (string, bool) { return "", false })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowedUserIDsEmptyAfterParse(t *testing.T) {
	t.Parallel()
	_, err := AllowedUserIDs(func(string) (string, bool) { return " , ", true })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowedUserIDsSetButEmptyString(t *testing.T) {
	t.Parallel()
	_, err := AllowedUserIDs(func(string) (string, bool) { return "", true })
	if err == nil {
		t.Fatal("expected error")
	}
}
