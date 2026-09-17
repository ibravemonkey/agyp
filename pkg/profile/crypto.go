package profile

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MagicHeader is the 8-byte file signature identifying an encrypted agyp archive.
var MagicHeader = []byte("AGYP_ENC")

// CurrentCryptoVersion represents the container version.
const CurrentCryptoVersion byte = 0x01

// PBKDF2Iterations defines the work factor for key derivation (OWASP standard).
const PBKDF2Iterations = 100000

// SaltLength is 16 bytes (128-bit) random salt.
const SaltLength = 16

// NonceLength is 12 bytes (96-bit) standard for AES-GCM.
const NonceLength = 12

// KeyLength is 32 bytes (256-bit) for AES-256.
const KeyLength = 32

// HeaderLength is Magic (8B) + Version (1B) + Salt (16B) + Nonce (12B) = 37 bytes.
const HeaderLength = len("AGYP_ENC") + 1 + SaltLength + NonceLength

// ErrInvalidPassword is returned when decryption fails due to wrong password or corrupted archive.
var ErrInvalidPassword = errors.New("invalid password or corrupted archive (authentication failed)")

// ErrNotEncrypted is returned when data does not have the AGYP_ENC magic header.
var ErrNotEncrypted = errors.New("archive is not encrypted (missing AGYP_ENC header)")

// DeriveKeyPBKDF2 derives a 256-bit AES key from a password and salt using PBKDF2-HMAC-SHA256 (RFC 2898).
func DeriveKeyPBKDF2(password []byte, salt []byte, iter int, keyLen int) []byte {
	numBlocks := (keyLen + sha256.Size - 1) / sha256.Size
	key := make([]byte, 0, numBlocks*sha256.Size)

	for block := 1; block <= numBlocks; block++ {
		h := hmac.New(sha256.New, password)
		h.Write(salt)
		var blockBytes [4]byte
		binary.BigEndian.PutUint32(blockBytes[:], uint32(block))
		h.Write(blockBytes[:])
		u := h.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)

		for i := 1; i < iter; i++ {
			h.Reset()
			h.Write(u)
			u = h.Sum(nil)
			for j := 0; j < len(t); j++ {
				t[j] ^= u[j]
			}
		}
		key = append(key, t...)
	}

	return key[:keyLen]
}

// EncryptData encrypts plaintext with AES-256-GCM using a key derived from password via PBKDF2.
// The output contains: [8B Magic] [1B Version] [16B Salt] [12B Nonce] [Ciphertext + 16B GCM Tag].
func EncryptData(plaintext []byte, password string) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("encryption password cannot be empty")
	}

	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate random salt: %w", err)
	}

	nonce := make([]byte, NonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	key := DeriveKeyPBKDF2([]byte(password), salt, PBKDF2Iterations, KeyLength)
	defer wipeSlice(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCM mode: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	var out bytes.Buffer
	out.Grow(HeaderLength + len(ciphertext))
	out.Write(MagicHeader)
	out.WriteByte(CurrentCryptoVersion)
	out.Write(salt)
	out.Write(nonce)
	out.Write(ciphertext)

	return out.Bytes(), nil
}

// DecryptData decrypts an AGYP_ENC container with AES-256-GCM using the provided password.
// Authenticates integrity before returning plaintext; returns ErrInvalidPassword on mismatch.
func DecryptData(encData []byte, password string) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("decryption password cannot be empty")
	}

	if len(encData) < HeaderLength {
		return nil, ErrNotEncrypted
	}

	if !bytes.Equal(encData[:len(MagicHeader)], MagicHeader) {
		return nil, ErrNotEncrypted
	}

	version := encData[len(MagicHeader)]
	if version != CurrentCryptoVersion {
		return nil, fmt.Errorf("unsupported encryption format version 0x%02x (expected 0x%02x)", version, CurrentCryptoVersion)
	}

	offset := len(MagicHeader) + 1
	salt := encData[offset : offset+SaltLength]
	offset += SaltLength

	nonce := encData[offset : offset+NonceLength]
	offset += NonceLength

	ciphertext := encData[offset:]

	key := DeriveKeyPBKDF2([]byte(password), salt, PBKDF2Iterations, KeyLength)
	defer wipeSlice(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCM mode: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidPassword
	}

	return plaintext, nil
}

// IsEncryptedArchive inspects the reader's first bytes to check if it's an AGYP_ENC container.
// It returns true if encrypted, along with a new io.Reader positioned at the beginning.
func IsEncryptedArchive(r io.Reader) (bool, io.Reader, error) {
	magicBuf := make([]byte, len(MagicHeader))
	n, err := io.ReadFull(r, magicBuf)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, bytes.NewReader(magicBuf[:n]), nil
		}
		return false, nil, err
	}

	isEnc := bytes.Equal(magicBuf, MagicHeader)
	// Reconstruct reader so no bytes are consumed
	newReader := io.MultiReader(bytes.NewReader(magicBuf), r)
	return isEnc, newReader, nil
}

// wipeSlice clears memory of sensitive cryptographic keys.
func wipeSlice(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ExportProfileEncrypted exports a profile to an encrypted AGYP_ENC archive.
func ExportProfileEncrypted(profileName string, writer io.Writer, password string) error {
	var tarBuf bytes.Buffer
	if err := ExportProfile(profileName, &tarBuf); err != nil {
		return err
	}

	encBytes, err := EncryptData(tarBuf.Bytes(), password)
	if err != nil {
		return err
	}

	_, err = writer.Write(encBytes)
	return err
}

// ExportAllEncrypted exports all profiles to an encrypted AGYP_ENC archive.
func ExportAllEncrypted(writer io.Writer, password string) error {
	var tarBuf bytes.Buffer
	if err := ExportAll(&tarBuf); err != nil {
		return err
	}

	encBytes, err := EncryptData(tarBuf.Bytes(), password)
	if err != nil {
		return err
	}

	_, err = writer.Write(encBytes)
	return err
}
