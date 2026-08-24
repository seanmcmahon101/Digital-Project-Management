package web

import "testing"

func TestNormaliseHexColor(t *testing.T) {
	tests := map[string]string{
		"#123abc": "#123ABC",
		"#FFFFFF": "#FFFFFF",
		"bad":     "#5C1E30",
		"#12345Z": "#5C1E30",
	}
	for in, want := range tests {
		if got := normaliseHexColor(in); got != want {
			t.Errorf("normaliseHexColor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSidebarColourDerivatives(t *testing.T) {
	if got := shadeHexColor("#FFFFFF", 0.78); got != "#C6C6C6" {
		t.Fatalf("shadeHexColor() = %q", got)
	}
	if got := readableForeground("#F7E64A"); got != "#211D1F" {
		t.Fatalf("foreground for light colour = %q", got)
	}
	if got := readableForeground("#002060"); got != "#FFFFFF" {
		t.Fatalf("foreground for dark colour = %q", got)
	}
}
