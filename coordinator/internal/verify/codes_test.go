package verify

import "testing"

func TestValidateCombo(t *testing.T) {
	cases := []struct {
		channel, binding string
		ok               bool
	}{
		{"sms", "derive", true},
		{"sms", "claim", true},
		{"dtmf", "derive", true},
		{"dtmf", "claim", true},
		{"call", "claim", true},
		{"call", "derive", false}, // missed call has no code -> claim only
		{"voice", "derive", false},
		{"sms", "whatever", false},
	}
	for _, c := range cases {
		err := validateCombo(c.channel, c.binding)
		if (err == nil) != c.ok {
			t.Errorf("validateCombo(%q,%q): got err=%v, want ok=%v", c.channel, c.binding, err, c.ok)
		}
	}
}

func TestChannelNeedsCode(t *testing.T) {
	if !channelNeedsCode("sms") || !channelNeedsCode("dtmf") {
		t.Error("sms and dtmf must need a code")
	}
	if channelNeedsCode("call") {
		t.Error("missed call must not need a code")
	}
}

func TestGenerateCode(t *testing.T) {
	c, err := generateCode(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 6 {
		t.Fatalf("len = %d, want 6", len(c))
	}
	for _, r := range c {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit in code %q", c)
		}
	}
}

func TestExtractCode(t *testing.T) {
	cases := map[string]string{
		"123456":              "123456",
		"your code is 482913": "482913",
		"123 456":             "123456",
		"hi":                  "",
		"12":                  "",
		"  4827 ":             "4827",
	}
	for in, want := range cases {
		if got := extractCode(in); got != want {
			t.Errorf("extractCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildInstructions(t *testing.T) {
	action, link := buildInstructions("sms", "017000", "4729")
	if action == "" || link != "sms:017000?body=4729" {
		t.Errorf("sms instructions wrong: %q %q", action, link)
	}
	_, link = buildInstructions("call", "017000", "")
	if link != "tel:017000" {
		t.Errorf("call deep link wrong: %q", link)
	}
}
