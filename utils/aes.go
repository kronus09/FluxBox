package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"strings"
)

// DecryptCBC 强制 16 字节对齐解密
func DecryptCBC(dataHex, keyStr, ivStr string) (string, error) {
	data, err := hex.DecodeString(strings.TrimSpace(dataHex))
	if err != nil {
		return "", err
	}

	// 强制构建 16 字节 Key/IV
	key := make([]byte, 16)
	copy(key, []byte(strings.TrimSpace(keyStr)))
	iv := make([]byte, 16)
	copy(iv, []byte(strings.TrimSpace(ivStr)))

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(data))
	blockMode.CryptBlocks(decrypted, data)

	// PKCS7UnPadding
	length := len(decrypted)
	if length > 0 {
		unpadding := int(decrypted[length-1])
		if unpadding > 0 && unpadding <= 16 && unpadding <= length {
			decrypted = decrypted[:(length - unpadding)]
		}
	}

	return strings.TrimSpace(string(decrypted)), nil
}

// DecryptECB 同步逻辑
func DecryptECB(dataHex, keyStr string) (string, error) {
	data, err := hex.DecodeString(strings.TrimSpace(dataHex))
	if err != nil {
		return "", err
	}

	if len(data)%16 != 0 {
		data = data[:len(data)-len(data)%16]
	}

	key := make([]byte, 16)
	copy(key, []byte(strings.TrimSpace(keyStr)))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	decrypted := make([]byte, len(data))
	size := block.BlockSize()
	for bs, be := 0, size; bs < len(data); bs, be = bs+size, be+size {
		block.Decrypt(decrypted[bs:be], data[bs:be])
	}
	// 去填充
	length := len(decrypted)
	if length > 0 {
		unpadding := int(decrypted[length-1])
		if unpadding > 0 && unpadding <= 16 && unpadding <= length {
			decrypted = decrypted[:(length - unpadding)]
		}
	}
	return strings.TrimSpace(string(decrypted)), nil
}
