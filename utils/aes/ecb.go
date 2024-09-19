type CryptoDB struct {
	block cipher.Block
}

// NewCryptoDB 创建新的 CryptoDB 实例并初始化 AES 加密块
func NewCryptoDB(key string) *CryptoDB {
	key := []byte(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	return &CryptoDB{block: block}
}

// Encrypt 对文本进行加密并返回加密后的 base64 编码字符串
func (c *CryptoDB) Encrypt(text string) string {
	paddedText := addTo16([]byte(text))
	encrypted := make([]byte, len(paddedText))
	for bs, be := 0, c.block.BlockSize(); bs < len(paddedText); bs, be = bs+c.block.BlockSize(), be+c.block.BlockSize() {
		c.block.Encrypt(encrypted[bs:be], paddedText[bs:be])
	}
	encMsg := base64.StdEncoding.EncodeToString(encrypted)
	return encMsg
}

// Decrypt 对 base64 编码的加密字符串进行解密并返回明文
func (c *CryptoDB) Decrypt(text string) string {
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		panic(err)
	}
	decrypted := make([]byte, len(decoded))
	for bs, be := 0, c.block.BlockSize(); bs < len(decoded); bs, be = bs+c.block.BlockSize(), be+c.block.BlockSize() {
		c.block.Decrypt(decrypted[bs:be], decoded[bs:be])
	}
	decMsg := string(decrypted)
	decMsg = strings.TrimRight(decMsg, "\x00") // 去掉填充的 \x00
	return decMsg
}

// addTo16 用于将明文填充到 16 字节的倍数
func addTo16(text []byte) []byte {
	padding := 16 - len(text)%16
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = '\x00'
	}
	return append(text, padtext...)
}
