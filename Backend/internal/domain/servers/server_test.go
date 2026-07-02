package servers

import "testing"

func TestParseConnectionStringSupportsBasisClientFormat(t *testing.T) {
	endpoint, err := ParseConnectionString("basis.example.org:4297#secret pass")
	if err != nil {
		t.Fatalf("ParseConnectionString returned error: %v", err)
	}

	if endpoint.Host != "basis.example.org" {
		t.Fatalf("Host = %q, want basis.example.org", endpoint.Host)
	}
	if endpoint.Port != 4297 {
		t.Fatalf("Port = %d, want 4297", endpoint.Port)
	}
	if endpoint.Password == nil || *endpoint.Password != "secret pass" {
		t.Fatalf("Password = %#v, want secret pass", endpoint.Password)
	}
}

func TestParseConnectionStringDefaultsPortAndStripsIPv6Brackets(t *testing.T) {
	endpoint, err := ParseConnectionString("[2001:4860:4860::8888]#pw")
	if err != nil {
		t.Fatalf("ParseConnectionString returned error: %v", err)
	}

	if endpoint.Host != "2001:4860:4860::8888" {
		t.Fatalf("Host = %q, want IPv6 literal without brackets", endpoint.Host)
	}
	if endpoint.Port != DefaultPort {
		t.Fatalf("Port = %d, want %d", endpoint.Port, DefaultPort)
	}
	if endpoint.Password == nil || *endpoint.Password != "pw" {
		t.Fatalf("Password = %#v, want pw", endpoint.Password)
	}
}

func TestEndpointPublicConnectionStringOmitsBlankPassword(t *testing.T) {
	endpoint := Endpoint{Host: "play.example.org", Port: DefaultPort}
	if got := endpoint.ConnectionString(); got != "play.example.org:4296" {
		t.Fatalf("ConnectionString = %q, want play.example.org:4296", got)
	}

	password := "secret"
	endpoint.Password = &password
	if got := endpoint.ConnectionString(); got != "play.example.org:4296#secret" {
		t.Fatalf("ConnectionString with password = %q, want play.example.org:4296#secret", got)
	}
}

func TestValidateEndpointRejectsPrivateAndLoopbackHosts(t *testing.T) {
	rejected := []Endpoint{
		{Host: "127.0.0.1", Port: DefaultPort},
		{Host: "::1", Port: DefaultPort},
		{Host: "10.0.0.10", Port: DefaultPort},
		{Host: "172.16.0.10", Port: DefaultPort},
		{Host: "192.168.1.10", Port: DefaultPort},
		{Host: "169.254.10.10", Port: DefaultPort},
		{Host: "localhost", Port: DefaultPort},
	}

	for _, endpoint := range rejected {
		if err := ValidateEndpointForPublicCheck(endpoint); err == nil {
			t.Fatalf("ValidateEndpointForPublicCheck(%+v) returned nil, want rejection", endpoint)
		}
	}
}

func TestNormalizeLanguageAcceptsLanguageTagsAndAliases(t *testing.T) {
	cases := map[string]string{
		"":             "",
		" EN ":         "en",
		"pt_BR":        "pt-br",
		"zh-cn":        "zh-cn",
		"English":      "en",
		"русский":      "ru",
		"Multilingual": "multi",
	}

	for input, want := range cases {
		got, err := NormalizeLanguage(input)
		if err != nil {
			t.Fatalf("NormalizeLanguage(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeLanguageRejectsUnstructuredValues(t *testing.T) {
	rejected := []string{
		"anything goes",
		"e",
		"english server",
		"en-",
		"en-verylongpart",
		"ru!",
	}

	for _, input := range rejected {
		if _, err := NormalizeLanguage(input); err == nil {
			t.Fatalf("NormalizeLanguage(%q) returned nil, want rejection", input)
		}
	}
}
