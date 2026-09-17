package profile

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// RFC 6070 test vector for PBKDF2-HMAC-SHA256
// Password = "password", Salt = "salt", c = 1, dkLen = 32
func TestPBKDF2_RFC6070(t *testing.T) {
	password := []byte("password")
	salt := []byte("salt")
	iter := 1
	keyLen := 32
	expectedHex := "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	key := DeriveKeyPBKDF2(password, salt, iter, keyLen)
	gotHex := hex.EncodeToString(key)
	if gotHex != expectedHex {
		t.Fatalf("PBKDF2 RFC 6070 mismatch: got %s, expected %s", gotHex, expectedHex)
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plaintext := []byte("Sensitive OAuth tokens and credentials for Antigravity profile")
	password := "Correct-Horse-Battery-Staple-2026!"

	encData, err := EncryptData(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}

	if len(encData) <= len(plaintext) {
		t.Fatalf("Ciphertext too short: %d bytes", len(encData))
	}

	// Decrypt with correct password
	decrypted, err := DecryptData(encData, password)
	if err != nil {
		t.Fatalf("DecryptData failed with correct password: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("Decrypted text mismatch: got %q, expected %q", string(decrypted), string(plaintext))
	}

	// Decrypt with wrong password must fail
	_, err = DecryptData(encData, "Wrong-Password-123")
	if err != ErrInvalidPassword {
		t.Fatalf("Expected ErrInvalidPassword on wrong password, got: %v", err)
	}
}

func TestEncryptDecrypt_TamperedCiphertext(t *testing.T) {
	plaintext := []byte("Secret payload")
	password := "securePass123"

	encData, err := EncryptData(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}

	// Tamper with last byte (part of authentication tag)
	tampered := make([]byte, len(encData))
	copy(tampered, encData)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = DecryptData(tampered, password)
	if err != ErrInvalidPassword {
		t.Fatalf("Expected ErrInvalidPassword on tampered ciphertext, got: %v", err)
	}
}

func TestIsEncryptedArchive(t *testing.T) {
	plaintextData := []byte("This is a standard tar.gz plaintext file data")
	isEnc, r, err := IsEncryptedArchive(bytes.NewReader(plaintextData))
	if err != nil {
		t.Fatalf("IsEncryptedArchive failed on plaintext: %v", err)
	}
	if isEnc {
		t.Fatalf("Expected isEnc=false for plaintext data")
	}

	// Read all from r to ensure no bytes lost
	readBack := new(bytes.Buffer)
	_, _ = readBack.ReadFrom(r)
	if !bytes.Equal(readBack.Bytes(), plaintextData) {
		t.Fatalf("IsEncryptedArchive lost bytes: got %q", readBack.String())
	}

	// Now test encrypted data
	encData, err := EncryptData([]byte("test"), "mypass")
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}

	isEnc, rEnc, err := IsEncryptedArchive(bytes.NewReader(encData))
	if err != nil {
		t.Fatalf("IsEncryptedArchive failed on encData: %v", err)
	}
	if !isEnc {
		t.Fatalf("Expected isEnc=true for encData")
	}

	readEncBack := new(bytes.Buffer)
	_, _ = readEncBack.ReadFrom(rEnc)
	if !bytes.Equal(readEncBack.Bytes(), encData) {
		t.Fatalf("IsEncryptedArchive lost bytes on encData")
	}
}

func TestExportProfileEncrypted_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	profileName := "cryptoexport"
	profileDir := filepath.Join(tempDir, ".agyp", "profiles", profileName)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	// Write dummy secret file
	secretFile := filepath.Join(profileDir, "credentials.json")
	if err := os.WriteFile(secretFile, []byte(`{"oauth_token":"super_secret_123"}`), 0600); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	password := "StrongP@ssw0rd!2026"
	var archiveBuf bytes.Buffer

	// Export encrypted
	if err := ExportProfileEncrypted(profileName, &archiveBuf, password); err != nil {
		t.Fatalf("ExportProfileEncrypted failed: %v", err)
	}

	// Check IsEncryptedArchive
	isEnc, encReader, err := IsEncryptedArchive(bytes.NewReader(archiveBuf.Bytes()))
	if err != nil || !isEnc {
		t.Fatalf("expected encrypted archive signature: isEnc=%v, err=%v", isEnc, err)
	}

	// Decrypt
	encBytes := new(bytes.Buffer)
	_, _ = encBytes.ReadFrom(encReader)
	decryptedTar, err := DecryptData(encBytes.Bytes(), password)
	if err != nil {
		t.Fatalf("DecryptData failed: %v", err)
	}

	// Import decrypted into a new profile
	importedProfileName := "cryptoimported"
	if err := ImportProfile(bytes.NewReader(decryptedTar), importedProfileName, true); err != nil {
		t.Fatalf("ImportProfile failed on decrypted tar: %v", err)
	}

	// Verify imported file
	importedFile := filepath.Join(tempDir, ".agyp", "profiles", importedProfileName, "credentials.json")
	content, err := os.ReadFile(importedFile)
	if err != nil {
		t.Fatalf("failed to read imported file: %v", err)
	}
	if !bytes.Contains(content, []byte("super_secret_123")) {
		t.Fatalf("imported file content mismatch: %s", string(content))
	}
}
