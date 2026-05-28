package basisbee

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"testing"
)

func TestValidateRealAssetFromEnv(t *testing.T) {
	path := os.Getenv("BEEBA_BASIS_BEE_FIXTURE")
	password := os.Getenv("BEEBA_BASIS_BEE_PASSWORD")
	if path == "" || password == "" {
		t.Skip("set BEEBA_BASIS_BEE_FIXTURE and BEEBA_BASIS_BEE_PASSWORD to validate a real Basis .bee asset")
	}

	maxBytes := int64(MaxSectionBytes + MaxConnectorBytes + RemoteHeaderSize)
	if raw := os.Getenv("BEEBA_BASIS_BEE_MAX_BYTES"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			t.Fatalf("parse BEEBA_BASIS_BEE_MAX_BYTES: %v", err)
		}
		maxBytes = parsed
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	result, encryptedConnector, err := readRemoteSDKBEE(file, maxBytes)
	if err != nil {
		t.Fatalf("read real Basis .bee fixture: %v", err)
	}
	plainConnector, err := decryptBasisBytes(password, encryptedConnector)
	if err != nil {
		t.Fatalf("decrypt real Basis .bee fixture connector: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(plainConnector, &raw); err != nil {
		t.Fatalf("unmarshal real Basis .bee fixture connector keys: %v", err)
	}
	t.Logf("real Basis .bee connector keys: %v", sortedKeys(raw))

	connector, err := parseConnectorJSON(plainConnector)
	if err != nil {
		t.Fatalf("unmarshal real Basis .bee fixture connector: %v", err)
	}
	if err := validateConnector(connector, result.SectionBytes); err != nil {
		t.Fatalf("validate real Basis .bee fixture connector: %v", err)
	}
	if result.Size <= 0 {
		t.Fatal("expected positive fixture size")
	}
	if result.SHA256 == "" {
		t.Fatal("expected fixture SHA-256")
	}
	if connector.UniqueVersion == "" {
		t.Fatal("expected connector unique version")
	}
	if len(connector.BasisBundleGenerated) == 0 {
		t.Fatal("expected at least one generated platform section")
	}
	result.UniqueVersion = connector.UniqueVersion
	result.AssetName = connector.BasisBundleDescription.AssetBundleName
	result.PlatformCount = len(connector.BasisBundleGenerated)
	if len(connector.BasisBundleGenerated) > 0 {
		result.AssetMode = connector.BasisBundleGenerated[0].AssetMode
	}
	t.Logf("validated real Basis .bee fixture: size=%d connector=%d sections=%d asset=%q mode=%q platforms=%d sha256=%s",
		result.Size,
		result.ConnectorBytes,
		result.SectionBytes,
		result.AssetName,
		result.AssetMode,
		result.PlatformCount,
		result.SHA256,
	)
}

func sortedKeys(raw map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
