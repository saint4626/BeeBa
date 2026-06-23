package basisbee

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"crypto/sha1"
	"golang.org/x/crypto/pbkdf2"
)

const (
	RemoteHeaderSize    = 8
	DiskHeaderSize      = 4
	MaxConnectorBytes   = 64 * 1024 * 1024
	MaxSectionBytes     = 4 * 1024 * 1024 * 1024
	encryptedMinLength  = 16 + 16 + 1
	maxPlatforms        = 16
	maxStringBytes      = 512
	maxDescriptionBytes = 8192
	maxTags             = 64
	maxTagBytes         = 64
	maxGraphicsAPIs     = 16
	maxComponents       = 256
)

type Result struct {
	Size            int64  `json:"size"`
	SHA256          string `json:"sha256"`
	ConnectorBytes  int64  `json:"connector_bytes"`
	SectionBytes    int64  `json:"section_bytes"`
	HeaderBytes     int    `json:"header_bytes"`
	RemoteSDKFormat bool   `json:"remote_sdk_format"`
	UniqueVersion   string `json:"unique_version,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
	AssetMode       string `json:"asset_mode,omitempty"`
	PlatformCount   int    `json:"platform_count,omitempty"`
}

type Connector struct {
	UniqueVersion          string      `json:"UniqueVersion"`
	BasisBundleDescription Description `json:"BasisBundleDescription"`
	BasisBundleGenerated   []Generated `json:"BasisBundleGenerated"`
	ImageBase64            string      `json:"ImageBase64"`
	DateOfCreation         string      `json:"DateOfCreation"`
	MetaData               Metadata    `json:"MetaData"`
}

func (connector *Connector) UnmarshalJSON(data []byte) error {
	var raw struct {
		UniqueVersion          string          `json:"UniqueVersion"`
		BasisBundleDescription Description     `json:"BasisBundleDescription"`
		BasisBundleGenerated   json.RawMessage `json:"BasisBundleGenerated"`
		ImageBase64            string          `json:"ImageBase64"`
		DateOfCreation         string          `json:"DateOfCreation"`
		MetaData               Metadata        `json:"MetaData"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	generated, err := parseGeneratedSections(raw.BasisBundleGenerated)
	if err != nil {
		return err
	}
	connector.UniqueVersion = raw.UniqueVersion
	connector.BasisBundleDescription = raw.BasisBundleDescription
	connector.BasisBundleGenerated = generated
	connector.ImageBase64 = raw.ImageBase64
	connector.DateOfCreation = raw.DateOfCreation
	connector.MetaData = raw.MetaData
	return nil
}

type Description struct {
	AssetBundleName        string   `json:"AssetBundleName"`
	AssetBundleDescription string   `json:"AssetBundleDescription"`
	Tags                   []string `json:"Tags"`
}

type Generated struct {
	AssetBundleHash string   `json:"AssetBundleHash"`
	AssetMode       string   `json:"AssetMode"`
	AssetToLoadName string   `json:"AssetToLoadName"`
	AssetBundleCRC  uint32   `json:"AssetBundleCRC"`
	IsEncrypted     bool     `json:"IsEncrypted"`
	Password        string   `json:"Password"`
	Platform        string   `json:"Platform"`
	EndByte         int64    `json:"EndByte"`
	GraphicsAPIs    []string `json:"GraphicsAPIs"`
}

type Metadata struct {
	TrianglesCount     int64           `json:"TrianglesCount"`
	MaterialCount      int64           `json:"MaterialCount"`
	BonesCount         int64           `json:"BonesCount"`
	TextureMemoryBytes int64           `json:"TextureMemoryBytes"`
	GraphicsPipeline   string          `json:"GraphicsPipeline"`
	ComponentNames     []ComponentName `json:"ComponentNames"`
}

type ComponentName struct {
	Name  string `json:"Name"`
	Count int    `json:"count"`
}

func ValidateRemoteSDKBEE(reader io.Reader, maxBytes int64) (Result, error) {
	result, _, err := readRemoteSDKBEE(reader, maxBytes)
	return result, err
}

func ValidateRemoteSDKBEEWithPassword(reader io.Reader, password string, maxBytes int64) (Result, Connector, error) {
	result, encryptedConnector, err := readRemoteSDKBEE(reader, maxBytes)
	if err != nil {
		return Result{}, Connector{}, err
	}
	plainConnector, err := decryptBasisBytes(password, encryptedConnector)
	if err != nil {
		return Result{}, Connector{}, fmt.Errorf("basis bee connector decrypt: %w", err)
	}
	connector, err := parseConnectorJSON(plainConnector)
	if err != nil {
		return Result{}, Connector{}, fmt.Errorf("basis bee connector json: %w", err)
	}
	if err := validateConnector(connector, result.SectionBytes); err != nil {
		return Result{}, Connector{}, err
	}
	result.UniqueVersion = connector.UniqueVersion
	result.AssetName = connector.BasisBundleDescription.AssetBundleName
	result.PlatformCount = len(connector.BasisBundleGenerated)
	if len(connector.BasisBundleGenerated) > 0 {
		result.AssetMode = connector.BasisBundleGenerated[0].AssetMode
	}
	return result, connector, nil
}

func parseConnectorJSON(data []byte) (Connector, error) {
	var connector Connector
	if err := json.Unmarshal(data, &connector); err != nil {
		return Connector{}, err
	}
	if connector.BasisBundleGenerated != nil || connector.BasisBundleDescription.AssetBundleName != "" || connector.UniqueVersion != "" {
		return connector, nil
	}

	var wrapped struct {
		Value Connector `json:"Value"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return Connector{}, err
	}
	return wrapped.Value, nil
}

func parseGeneratedSections(data json.RawMessage) ([]Generated, error) {
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, nil
	}
	var generated []Generated
	if err := json.Unmarshal(data, &generated); err == nil {
		return generated, nil
	}
	var wrapped struct {
		RLength  int         `json:"$rlength"`
		RContent []Generated `json:"$rcontent"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.RContent != nil {
		if wrapped.RLength > 0 && wrapped.RLength != len(wrapped.RContent) {
			return nil, fmt.Errorf("basis bee generated section count mismatch: declared %d actual %d", wrapped.RLength, len(wrapped.RContent))
		}
		return wrapped.RContent, nil
	}
	var single Generated
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, err
	}
	return []Generated{single}, nil
}

func readRemoteSDKBEE(reader io.Reader, maxBytes int64) (Result, []byte, error) {
	hasher := sha256.New()
	counting := &countingReader{reader: io.TeeReader(reader, hasher)}

	header := make([]byte, RemoteHeaderSize)
	if _, err := io.ReadFull(counting, header); err != nil {
		return Result{}, nil, fmt.Errorf("basis bee header: %w", err)
	}

	connectorBytes := int64(binary.LittleEndian.Uint64(header))
	if connectorBytes <= 0 {
		return Result{}, nil, fmt.Errorf("basis bee connector length must be positive")
	}
	if connectorBytes > MaxConnectorBytes {
		return Result{}, nil, fmt.Errorf("basis bee connector length %d exceeds max %d", connectorBytes, MaxConnectorBytes)
	}
	if connectorBytes < encryptedMinLength {
		return Result{}, nil, fmt.Errorf("basis bee encrypted connector is too small")
	}
	if maxBytes > 0 && RemoteHeaderSize+connectorBytes > maxBytes {
		return Result{}, nil, fmt.Errorf("basis bee connector exceeds configured upload limit")
	}

	connectorBuffer := bytes.NewBuffer(make([]byte, 0, connectorBytes))
	connectorWritten, err := io.CopyN(connectorBuffer, counting, connectorBytes)
	if err != nil {
		return Result{}, nil, fmt.Errorf("basis bee connector: expected %d bytes, read %d: %w", connectorBytes, connectorWritten, err)
	}

	sectionBytes, err := io.Copy(io.Discard, counting)
	if err != nil {
		return Result{}, nil, fmt.Errorf("basis bee section bytes: %w", err)
	}
	if sectionBytes <= 0 {
		return Result{}, nil, fmt.Errorf("basis bee has no bundle section bytes")
	}
	if sectionBytes > MaxSectionBytes {
		return Result{}, nil, fmt.Errorf("basis bee section bytes %d exceeds max %d", sectionBytes, int64(MaxSectionBytes))
	}
	if maxBytes > 0 && counting.n > maxBytes {
		return Result{}, nil, fmt.Errorf("basis bee size %d exceeds configured upload limit %d", counting.n, maxBytes)
	}

	return Result{
		Size:            counting.n,
		SHA256:          hex.EncodeToString(hasher.Sum(nil)),
		ConnectorBytes:  connectorBytes,
		SectionBytes:    sectionBytes,
		HeaderBytes:     RemoteHeaderSize,
		RemoteSDKFormat: true,
	}, connectorBuffer.Bytes(), nil
}

func decryptBasisBytes(password string, encrypted []byte) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(encrypted) < encryptedMinLength {
		return nil, fmt.Errorf("encrypted data is too short")
	}
	salt := encrypted[:16]
	iv := encrypted[16:32]
	ciphertext := encrypted[32:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not aes block aligned")
	}
	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha1.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext, aes.BlockSize)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid pkcs7 data length")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid pkcs7 padding")
	}
	var invalid byte
	for _, value := range data[len(data)-padding:] {
		invalid |= byte(subtle.ConstantTimeByteEq(value, byte(padding)) ^ 1)
	}
	if invalid != 0 {
		return nil, fmt.Errorf("invalid pkcs7 padding")
	}
	return data[:len(data)-padding], nil
}

func validateConnector(connector Connector, sectionBytes int64) error {
	if err := validateString("UniqueVersion", connector.UniqueVersion, maxStringBytes, true); err != nil {
		return err
	}
	if err := validateString("AssetBundleName", connector.BasisBundleDescription.AssetBundleName, maxStringBytes, true); err != nil {
		return err
	}
	if err := validateString("AssetBundleDescription", connector.BasisBundleDescription.AssetBundleDescription, maxDescriptionBytes, false); err != nil {
		return err
	}
	if err := validateTags(connector.BasisBundleDescription.Tags); err != nil {
		return err
	}
	if len(connector.BasisBundleGenerated) == 0 {
		return fmt.Errorf("basis bee connector has no generated platform sections")
	}
	if len(connector.BasisBundleGenerated) > maxPlatforms {
		return fmt.Errorf("basis bee connector has too many platform sections")
	}
	if connector.MetaData.TrianglesCount < 0 || connector.MetaData.MaterialCount < 0 || connector.MetaData.BonesCount < 0 || connector.MetaData.TextureMemoryBytes < 0 {
		return fmt.Errorf("basis bee connector metadata counters must not be negative")
	}
	if err := validateString("GraphicsPipeline", connector.MetaData.GraphicsPipeline, maxStringBytes, false); err != nil {
		return err
	}
	if len(connector.MetaData.ComponentNames) > maxComponents {
		return fmt.Errorf("basis bee connector has too many component names")
	}
	for index, component := range connector.MetaData.ComponentNames {
		if err := validateString(fmt.Sprintf("ComponentNames[%d].Name", index), component.Name, maxStringBytes, false); err != nil {
			return err
		}
		if component.Count < 0 {
			return fmt.Errorf("basis bee connector component %d has negative count", index)
		}
	}
	seenPlatforms := make(map[string]struct{}, len(connector.BasisBundleGenerated))
	var declaredSections int64
	for index, generated := range connector.BasisBundleGenerated {
		if generated.EndByte <= 0 {
			return fmt.Errorf("basis bee connector section %d has invalid EndByte", index)
		}
		if generated.EndByte > MaxSectionBytes {
			return fmt.Errorf("basis bee connector section %d exceeds max section bytes", index)
		}
		if declaredSections > math.MaxInt64-generated.EndByte {
			return fmt.Errorf("basis bee connector section bytes overflow")
		}
		declaredSections += generated.EndByte
		if declaredSections > MaxSectionBytes {
			return fmt.Errorf("basis bee connector declared sections exceed max section bytes")
		}
		if err := validateString(fmt.Sprintf("section %d AssetBundleHash", index), generated.AssetBundleHash, maxStringBytes, true); err != nil {
			return err
		}
		if err := validateString(fmt.Sprintf("section %d AssetToLoadName", index), generated.AssetToLoadName, maxStringBytes, true); err != nil {
			return err
		}
		if err := validateString(fmt.Sprintf("section %d Platform", index), generated.Platform, maxStringBytes, true); err != nil {
			return err
		}
		if !allowedPlatform(generated.Platform) {
			return fmt.Errorf("basis bee connector section %d has unsupported Platform %q", index, generated.Platform)
		}
		platformKey := strings.ToLower(strings.TrimSpace(generated.Platform))
		if _, exists := seenPlatforms[platformKey]; exists {
			return fmt.Errorf("basis bee connector has duplicate Platform %q", generated.Platform)
		}
		seenPlatforms[platformKey] = struct{}{}
		if err := validateAssetMode(index, generated.AssetMode); err != nil {
			return err
		}
		if generated.IsEncrypted && strings.TrimSpace(generated.Password) == "" {
			return fmt.Errorf("basis bee connector section %d missing encrypted bundle password", index)
		}
		if len(generated.Password) > maxStringBytes {
			return fmt.Errorf("basis bee connector section %d password is too long", index)
		}
		if len(generated.GraphicsAPIs) > maxGraphicsAPIs {
			return fmt.Errorf("basis bee connector section %d has too many GraphicsAPIs", index)
		}
		for apiIndex, api := range generated.GraphicsAPIs {
			if err := validateString(fmt.Sprintf("section %d GraphicsAPIs[%d]", index, apiIndex), api, maxStringBytes, false); err != nil {
				return err
			}
		}
	}
	if declaredSections != sectionBytes {
		return fmt.Errorf("basis bee connector section bytes mismatch: declared %d actual %d", declaredSections, sectionBytes)
	}
	return nil
}

func validateString(name string, value string, maxBytes int, required bool) error {
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return fmt.Errorf("basis bee connector missing %s", name)
	}
	if value != "" && !utf8.ValidString(value) {
		return fmt.Errorf("basis bee connector %s is not valid utf-8", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("basis bee connector %s exceeds %d bytes", name, maxBytes)
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > maxTags {
		return fmt.Errorf("basis bee connector has too many tags")
	}
	seen := make(map[string]struct{}, len(tags))
	for index, tag := range tags {
		if err := validateString(fmt.Sprintf("tag %d", index), tag, maxTagBytes, false); err != nil {
			return err
		}
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("basis bee connector duplicate tag %q", trimmed)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateAssetMode(index int, mode string) error {
	trimmed := strings.TrimSpace(mode)
	if trimmed == "" {
		return fmt.Errorf("basis bee connector section %d missing AssetMode", index)
	}
	switch trimmed {
	case "Scene", "GameObject", "Gameobject":
		return nil
	default:
		return fmt.Errorf("basis bee connector section %d has unsupported AssetMode %q", index, mode)
	}
}

func allowedPlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case "StandaloneWindows", "StandaloneWindows64", "StandaloneOSX", "Android", "iOS", "StandaloneLinux64":
		return true
	default:
		return false
	}
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}
