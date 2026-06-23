package basisbee

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"crypto/sha1"

	"golang.org/x/crypto/pbkdf2"
)

func TestValidateRemoteSDKBEE(t *testing.T) {
	connector := bytes.Repeat([]byte{1}, encryptedMinLength)
	section := []byte("bundle-section")
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(connector)))
	input.Write(header)
	input.Write(connector)
	input.Write(section)

	result, err := ValidateRemoteSDKBEE(bytes.NewReader(input.Bytes()), 1024)
	if err != nil {
		t.Fatalf("ValidateRemoteSDKBEE returned error: %v", err)
	}
	if result.ConnectorBytes != int64(len(connector)) {
		t.Fatalf("connector bytes = %d, want %d", result.ConnectorBytes, len(connector))
	}
	if result.SectionBytes != int64(len(section)) {
		t.Fatalf("section bytes = %d, want %d", result.SectionBytes, len(section))
	}
	if len(result.SHA256) != 64 {
		t.Fatalf("sha256 length = %d, want 64", len(result.SHA256))
	}
}

func TestValidateRemoteSDKBEERejectsDiskHeaderShape(t *testing.T) {
	connector := bytes.Repeat([]byte{1}, encryptedMinLength)
	var input bytes.Buffer
	header := make([]byte, DiskHeaderSize)
	binary.LittleEndian.PutUint32(header, uint32(len(connector)))
	input.Write(header)
	input.Write(connector)
	input.WriteString("section")

	_, err := ValidateRemoteSDKBEE(bytes.NewReader(input.Bytes()), 1024)
	if err == nil {
		t.Fatal("expected disk-header shaped file to fail remote SDK validation")
	}
}

func TestValidateRemoteSDKBEERejectsMissingSection(t *testing.T) {
	connector := bytes.Repeat([]byte{1}, encryptedMinLength)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(connector)))
	input.Write(header)
	input.Write(connector)

	_, err := ValidateRemoteSDKBEE(bytes.NewReader(input.Bytes()), 1024)
	if err == nil || !strings.Contains(err.Error(), "no bundle section") {
		t.Fatalf("expected no section error, got %v", err)
	}
}

func TestValidateRemoteSDKBEEWithPassword(t *testing.T) {
	password := "test-password"
	section := []byte("bundle-section")
	connectorJSON, err := json.Marshal(Connector{
		UniqueVersion: "version-1",
		BasisBundleDescription: Description{
			AssetBundleName:        "Test World",
			AssetBundleDescription: "Test Description",
			Tags:                   []string{"test"},
		},
		BasisBundleGenerated: []Generated{
			{
				AssetBundleHash: "hash",
				AssetMode:       "Scene",
				AssetToLoadName: "Scene",
				IsEncrypted:     true,
				Password:        password,
				Platform:        "StandaloneWindows64",
				EndByte:         int64(len(section)),
			},
		},
		ImageBase64: "embedded-preview",
	})
	if err != nil {
		t.Fatalf("marshal connector: %v", err)
	}
	encryptedConnector := encryptForTest(t, password, connectorJSON)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(encryptedConnector)))
	input.Write(header)
	input.Write(encryptedConnector)
	input.Write(section)

	result, connector, err := ValidateRemoteSDKBEEWithPassword(bytes.NewReader(input.Bytes()), password, 4096)
	if err != nil {
		t.Fatalf("ValidateRemoteSDKBEEWithPassword returned error: %v", err)
	}
	if result.AssetName != "Test World" {
		t.Fatalf("asset name = %q", result.AssetName)
	}
	if connector.UniqueVersion != "version-1" {
		t.Fatalf("unique version = %q", connector.UniqueVersion)
	}
	if connector.ImageBase64 != "embedded-preview" {
		t.Fatalf("image base64 = %q", connector.ImageBase64)
	}
}

func TestValidateRemoteSDKBEEWithPasswordAcceptsBasisSerializationWrapper(t *testing.T) {
	password := "test-password"
	section := []byte("bundle-section")
	connectorJSON, err := json.Marshal(struct {
		Value Connector `json:"Value"`
	}{
		Value: testConnector(password, int64(len(section))),
	})
	if err != nil {
		t.Fatalf("marshal connector: %v", err)
	}
	encryptedConnector := encryptForTest(t, password, connectorJSON)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(encryptedConnector)))
	input.Write(header)
	input.Write(encryptedConnector)
	input.Write(section)

	result, connector, err := ValidateRemoteSDKBEEWithPassword(bytes.NewReader(input.Bytes()), password, 4096)
	if err != nil {
		t.Fatalf("ValidateRemoteSDKBEEWithPassword returned error: %v", err)
	}
	if result.AssetName != "Test World" {
		t.Fatalf("asset name = %q", result.AssetName)
	}
	if connector.UniqueVersion != "version-1" {
		t.Fatalf("unique version = %q", connector.UniqueVersion)
	}
}

func TestValidateRemoteSDKBEEWithPasswordAcceptsSingleGeneratedSectionObject(t *testing.T) {
	password := "test-password"
	section := []byte("bundle-section")
	base := testConnector(password, int64(len(section)))
	connectorJSON, err := json.Marshal(struct {
		UniqueVersion          string      `json:"UniqueVersion"`
		BasisBundleDescription Description `json:"BasisBundleDescription"`
		BasisBundleGenerated   Generated   `json:"BasisBundleGenerated"`
		DateOfCreation         string      `json:"DateOfCreation"`
		MetaData               Metadata    `json:"MetaData"`
	}{
		UniqueVersion:          base.UniqueVersion,
		BasisBundleDescription: base.BasisBundleDescription,
		BasisBundleGenerated:   base.BasisBundleGenerated[0],
		DateOfCreation:         base.DateOfCreation,
		MetaData:               base.MetaData,
	})
	if err != nil {
		t.Fatalf("marshal connector: %v", err)
	}
	encryptedConnector := encryptForTest(t, password, connectorJSON)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(encryptedConnector)))
	input.Write(header)
	input.Write(encryptedConnector)
	input.Write(section)

	result, connector, err := ValidateRemoteSDKBEEWithPassword(bytes.NewReader(input.Bytes()), password, 4096)
	if err != nil {
		t.Fatalf("ValidateRemoteSDKBEEWithPassword returned error: %v", err)
	}
	if result.PlatformCount != 1 {
		t.Fatalf("platform count = %d", result.PlatformCount)
	}
	if len(connector.BasisBundleGenerated) != 1 {
		t.Fatalf("generated sections = %d", len(connector.BasisBundleGenerated))
	}
}

func TestValidateRemoteSDKBEEWithPasswordAcceptsBasisGeneratedArrayObject(t *testing.T) {
	password := "test-password"
	section := []byte("bundle-section-onebundle-section-two")
	base := testConnector(password, int64(len("bundle-section-one")))
	second := base.BasisBundleGenerated[0]
	second.Platform = "StandaloneLinux64"
	second.EndByte = int64(len("bundle-section-two"))
	connectorJSON, err := json.Marshal(struct {
		UniqueVersion          string      `json:"UniqueVersion"`
		BasisBundleDescription Description `json:"BasisBundleDescription"`
		BasisBundleGenerated   struct {
			RLength  int         `json:"$rlength"`
			RContent []Generated `json:"$rcontent"`
		} `json:"BasisBundleGenerated"`
		DateOfCreation string   `json:"DateOfCreation"`
		MetaData       Metadata `json:"MetaData"`
	}{
		UniqueVersion:          base.UniqueVersion,
		BasisBundleDescription: base.BasisBundleDescription,
		BasisBundleGenerated: struct {
			RLength  int         `json:"$rlength"`
			RContent []Generated `json:"$rcontent"`
		}{
			RLength:  2,
			RContent: []Generated{base.BasisBundleGenerated[0], second},
		},
		DateOfCreation: base.DateOfCreation,
		MetaData:       base.MetaData,
	})
	if err != nil {
		t.Fatalf("marshal connector: %v", err)
	}
	encryptedConnector := encryptForTest(t, password, connectorJSON)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(encryptedConnector)))
	input.Write(header)
	input.Write(encryptedConnector)
	input.Write(section)

	result, connector, err := ValidateRemoteSDKBEEWithPassword(bytes.NewReader(input.Bytes()), password, 4096)
	if err != nil {
		t.Fatalf("ValidateRemoteSDKBEEWithPassword returned error: %v", err)
	}
	if result.PlatformCount != 2 {
		t.Fatalf("platform count = %d", result.PlatformCount)
	}
	if len(connector.BasisBundleGenerated) != 2 {
		t.Fatalf("generated sections = %d", len(connector.BasisBundleGenerated))
	}
}

func TestValidateRemoteSDKBEEWithPasswordAcceptsGameObjectAssetMode(t *testing.T) {
	_, _, err := ValidateRemoteSDKBEEWithPassword(
		bytes.NewReader(testEncryptedBEE(t, "test-password", []byte("bundle-section"), func(connector *Connector) {
			connector.BasisBundleGenerated[0].AssetMode = "GameObject"
			connector.BasisBundleGenerated[0].AssetToLoadName = "AvatarRoot"
		})),
		"test-password",
		4096,
	)
	if err != nil {
		t.Fatalf("expected GameObject AssetMode to be accepted, got %v", err)
	}
}

func TestValidateRemoteSDKBEEWithPasswordRejectsUnsupportedAssetMode(t *testing.T) {
	_, _, err := ValidateRemoteSDKBEEWithPassword(
		bytes.NewReader(testEncryptedBEE(t, "test-password", []byte("bundle-section"), func(connector *Connector) {
			connector.BasisBundleGenerated[0].AssetMode = "ScriptableObject"
		})),
		"test-password",
		4096,
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported AssetMode") {
		t.Fatalf("expected unsupported AssetMode error, got %v", err)
	}
}

func TestValidateRemoteSDKBEEWithPasswordRejectsDuplicatePlatforms(t *testing.T) {
	_, _, err := ValidateRemoteSDKBEEWithPassword(
		bytes.NewReader(testEncryptedBEE(t, "test-password", []byte("bundle-section-onebundle-section-two"), func(connector *Connector) {
			first := connector.BasisBundleGenerated[0]
			first.EndByte = int64(len("bundle-section-one"))
			second := first
			second.EndByte = int64(len("bundle-section-two"))
			connector.BasisBundleGenerated = []Generated{first, second}
		})),
		"test-password",
		4096,
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate Platform") {
		t.Fatalf("expected duplicate Platform error, got %v", err)
	}
}

func TestValidateRemoteSDKBEEWithPasswordRejectsTooManyTags(t *testing.T) {
	_, _, err := ValidateRemoteSDKBEEWithPassword(
		bytes.NewReader(testEncryptedBEE(t, "test-password", []byte("bundle-section"), func(connector *Connector) {
			connector.BasisBundleDescription.Tags = make([]string, maxTags+1)
			for index := range connector.BasisBundleDescription.Tags {
				connector.BasisBundleDescription.Tags[index] = "tag"
			}
		})),
		"test-password",
		8192,
	)
	if err == nil || !strings.Contains(err.Error(), "too many tags") {
		t.Fatalf("expected too many tags error, got %v", err)
	}
}

func TestValidateRemoteSDKBEEWithPasswordRejectsNegativeMetadata(t *testing.T) {
	_, _, err := ValidateRemoteSDKBEEWithPassword(
		bytes.NewReader(testEncryptedBEE(t, "test-password", []byte("bundle-section"), func(connector *Connector) {
			connector.MetaData.TrianglesCount = -1
		})),
		"test-password",
		4096,
	)
	if err == nil || !strings.Contains(err.Error(), "metadata counters") {
		t.Fatalf("expected metadata counter error, got %v", err)
	}
}

func TestValidateConnectorRejectsDeclaredSectionOverflow(t *testing.T) {
	connector := testConnector("test-password", 1)
	connector.BasisBundleGenerated = []Generated{
		{
			AssetBundleHash: "hash-1",
			AssetMode:       "Scene",
			AssetToLoadName: "Scene",
			IsEncrypted:     true,
			Password:        "test-password",
			Platform:        "StandaloneWindows64",
			EndByte:         MaxSectionBytes,
		},
		{
			AssetBundleHash: "hash-2",
			AssetMode:       "Scene",
			AssetToLoadName: "Scene",
			IsEncrypted:     true,
			Password:        "test-password",
			Platform:        "StandaloneLinux64",
			EndByte:         1,
		},
	}
	err := validateConnector(connector, MaxSectionBytes+1)
	if err == nil || !strings.Contains(err.Error(), "declared sections exceed") {
		t.Fatalf("expected section max error, got %v", err)
	}
}

func testEncryptedBEE(t *testing.T, password string, section []byte, mutate func(*Connector)) []byte {
	t.Helper()
	connector := testConnector(password, int64(len(section)))
	if mutate != nil {
		mutate(&connector)
	}
	connectorJSON, err := json.Marshal(connector)
	if err != nil {
		t.Fatalf("marshal connector: %v", err)
	}
	encryptedConnector := encryptForTest(t, password, connectorJSON)
	var input bytes.Buffer
	header := make([]byte, RemoteHeaderSize)
	binary.LittleEndian.PutUint64(header, uint64(len(encryptedConnector)))
	input.Write(header)
	input.Write(encryptedConnector)
	input.Write(section)
	return input.Bytes()
}

func testConnector(password string, sectionBytes int64) Connector {
	return Connector{
		UniqueVersion: "version-1",
		BasisBundleDescription: Description{
			AssetBundleName:        "Test World",
			AssetBundleDescription: "Test Description",
			Tags:                   []string{"test"},
		},
		BasisBundleGenerated: []Generated{
			{
				AssetBundleHash: "hash",
				AssetMode:       "Scene",
				AssetToLoadName: "Scene",
				IsEncrypted:     true,
				Password:        password,
				Platform:        "StandaloneWindows64",
				EndByte:         sectionBytes,
				GraphicsAPIs:    []string{"Direct3D11"},
			},
		},
		MetaData: Metadata{
			TrianglesCount:     1,
			MaterialCount:      1,
			BonesCount:         0,
			TextureMemoryBytes: 0,
			GraphicsPipeline:   "URP",
			ComponentNames: []ComponentName{
				{Name: "Transform", Count: 1},
			},
		},
	}
}

func encryptForTest(t *testing.T, password string, data []byte) []byte {
	t.Helper()
	salt := make([]byte, 16)
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		t.Fatalf("salt: %v", err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatalf("iv: %v", err)
	}
	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha1.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	padded := pkcs7PadForTest(data, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	return append(append(salt, iv...), ciphertext...)
}

func pkcs7PadForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}
