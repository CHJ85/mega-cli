package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProgressTracker struct {
	Reader    io.Reader
	Total     int64
	BytesRead int64
}

func (pt *ProgressTracker) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	pt.BytesRead += int64(n)
	fmt.Printf("\r -> Downloading: %d / %d bytes", pt.BytesRead, pt.Total)
	return n, err
}

func base64URLDecode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	for len(s)%4 != 0 {
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}

func parseMegaURL(url string) (string, string, error) {
	re := regexp.MustCompile(`mega\.nz/file/([^#]+)#([^#]+)`)
	match := re.FindStringSubmatch(url)
	if len(match) != 3 {
		reLegacy := regexp.MustCompile(`mega\.nz/#!([^!]+)!([^!]+)`)
		match = reLegacy.FindStringSubmatch(url)
		if len(match) != 3 {
			return "", "", fmt.Errorf("invalid mega URL format")
		}
	}
	return match[1], match[2], nil
}

func decryptAttributes(attrB64 string, keyWords []uint32) (string, error) {
	attrData, err := base64URLDecode(attrB64)
	if err != nil {
		return "", err
	}
	keyBytes := make([]byte, 16)
	binary.BigEndian.PutUint32(keyBytes[0:], keyWords[0]^keyWords[4])
	binary.BigEndian.PutUint32(keyBytes[4:], keyWords[1]^keyWords[5])
	binary.BigEndian.PutUint32(keyBytes[8:], keyWords[2]^keyWords[6])
	binary.BigEndian.PutUint32(keyBytes[12:], keyWords[3]^keyWords[7])

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	if len(attrData)%16 != 0 {
		pad := 16 - (len(attrData) % 16)
		attrData = append(attrData, bytes.Repeat([]byte{0}, pad)...)
	}
	iv := make([]byte, 16)
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(attrData))
	mode.CryptBlocks(decrypted, attrData)
	if !bytes.HasPrefix(decrypted, []byte("MEGA")) {
		return "", fmt.Errorf("metadata decryption failed")
	}
	cleanData := bytes.Split(decrypted[4:], []byte{0})[0]
	var meta map[string]interface{}
	if err := json.Unmarshal(cleanData, &meta); err != nil {
		return "", err
	}
	if name, ok := meta["n"].(string); ok {
		return name, nil
	}
	return "downloaded_file", nil
}

func processURL(url string, outputDir string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	fmt.Printf("\nProcessing: %s\n", url)
	fileID, b64Key, err := parseMegaURL(url)
	if err != nil {
		return err
	}
	keyData, err := base64URLDecode(b64Key)
	if err != nil {
		return err
	}
	keyWords := make([]uint32, 8)
	for i := 0; i < 8; i++ {
		keyWords[i] = binary.BigEndian.Uint32(keyData[i*4 : (i+1)*4])
	}
	apiURL := "https://g.api.mega.co.nz/cs"
	payload := []map[string]interface{}{{"a": "g", "g": 1, "p": fileID}}
	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var apiResp []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}
	fileSize := int64(apiResp[0]["s"].(float64))
	downloadURL := apiResp[0]["g"].(string)
	attrB64 := apiResp[0]["at"].(string)
	fileName, err := decryptAttributes(attrB64, keyWords)
	if err != nil {
		fileName = fmt.Sprintf("mega_download_%s.rar", fileID)
	}
	targetPath := filepath.Join(outputDir, fileName)
	fmt.Printf(" -> Saving to: %s\n", targetPath)

	keyBytes := make([]byte, 16)
	binary.BigEndian.PutUint32(keyBytes[0:], keyWords[0]^keyWords[4])
	binary.BigEndian.PutUint32(keyBytes[4:], keyWords[1]^keyWords[5])
	binary.BigEndian.PutUint32(keyBytes[8:], keyWords[2]^keyWords[6])
	binary.BigEndian.PutUint32(keyBytes[12:], keyWords[3]^keyWords[7])
	ivBytes := make([]byte, 16)
	binary.BigEndian.PutUint32(ivBytes[0:], keyWords[4])
	binary.BigEndian.PutUint32(ivBytes[4:], keyWords[5])
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return err
	}
	stream := cipher.NewCTR(block, ivBytes)
	outF, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outF.Close()
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer dlResp.Body.Close()
	streamReader := &cipher.StreamReader{S: stream, R: dlResp.Body}
	progressReader := &ProgressTracker{Reader: streamReader, Total: fileSize}
	_, err = io.Copy(outF, progressReader)
	fmt.Println("\n -> Done.")
	return err
}

func main() {
	outputDir := flag.String("o", ".", "Output directory")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: mega [URL1] [URL2] [...] [-o dir] or mega [file.txt]")
		os.Exit(1)
	}
	targetDir, _ := filepath.Abs(*outputDir)
	os.MkdirAll(targetDir, 0755)

	for _, arg := range args {
		if strings.HasSuffix(arg, ".txt") {
			file, err := os.Open(arg)
			if err != nil {
				fmt.Printf("Error opening %s: %v\n", arg, err)
				continue
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				processURL(scanner.Text(), targetDir)
			}
			file.Close()
		} else {
			processURL(arg, targetDir)
		}
	}
}
