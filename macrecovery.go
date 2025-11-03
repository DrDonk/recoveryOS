package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Mac identifiers
	RecentMac   = "Mac-27AD2F918AE68F61"
	MLBZero     = "00000000000000000"
	MLBValid    = "F5K105303J9K3F71M"
	MLBProduct  = "F5K00000000K3F700"

	// ID types
	TypeSID = 16
	TypeK   = 64
	TypeFG  = 64

	// Info keys
	InfoProduct    = "AP"
	InfoImageLink  = "AU"
	InfoImageHash  = "AH"
	InfoImageSess  = "AT"
	InfoSignLink   = "CU"
	InfoSignHash   = "CH"
	InfoSignSess   = "CT"

	// Terminal margin
	TerminalMargin = 2
)

var (
	AppleEFIROMPublicKey1 *big.Int
	infoRequired          = []string{InfoProduct, InfoImageLink, InfoImageHash, InfoImageSess, InfoSignLink, InfoSignHash, InfoSignSess}
)

func init() {
	// Initialize the RSA public key
	AppleEFIROMPublicKey1, _ = new(big.Int).SetString("C3E748CAD9CD384329E10E25A91E43E1A762FF529ADE578C935BDDF9B13F2179D4855E6FC89E9E29CA12517D17DFA1EDCE0BEBF0EA7B461FFE61D94E2BDF72C196F89ACD3536B644064014DAE25A15DB6BB0852ECBD120916318D1CCDEA3C84C92ED743FC176D0BACA920D3FCF3158AFF731F88CE0623182A8ED67E650515F75745909F07D415F55FC15A35654D118C55A462D37A3ACDA08612F3F3F6571761EFCCBCC299AEE99B3A4FD6212CCFFF5EF37A2C334E871191F7E1C31960E010A54E86FA3F62E6D6905E1CD57732410A3EB0C6B4DEFDABE9F59BF1618758C751CD56CEF851D1C0EAA1C558E37AC108DA9089863D20E2E7E4BF475EC66FE6B3EFDCF", 16)
	rand.Seed(time.Now().UnixNano())
}

type ChunkListHeader struct {
	Magic           [4]byte
	HeaderSize      uint32
	FileVersion     uint8
	ChunkMethod     uint8
	SignatureMethod uint8
	_               uint8
	ChunkCount      uint64
	ChunkOffset     uint64
	SignatureOffset uint64
}

type Chunk struct {
	Size uint32
	Hash [32]byte
}

func runQuery(urlStr string, headers map[string]string, post map[string]string, raw bool) (http.Header, []byte, *http.Response, error) {
	var req *http.Request
	var err error

	if post != nil {
		var parts []string
		for k, v := range post {
			parts = append(parts, k+"="+v)
		}
		body := strings.Join(parts, "\n")
		req, err = http.NewRequest("POST", urlStr, strings.NewReader(body))
	} else {
		req, err = http.NewRequest("GET", urlStr, nil)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}

	if raw {
		return nil, nil, resp, nil
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Header, data, nil, nil
}

func generateID(idType int, idValue string) string {
	if idValue != "" {
		return idValue
	}
	const hexDigits = "0123456789ABCDEF"
	result := make([]byte, idType)
	for i := 0; i < idType; i++ {
		result[i] = hexDigits[rand.Intn(16)]
	}
	return string(result)
}

func productMLB(mlb string) string {
	if len(mlb) < 17 {
		return mlb
	}
	return "00000000000" + mlb[11:15] + "00"
}

func mlbFromEEEE(eeee string) (string, error) {
	if len(eeee) != 4 {
		return "", fmt.Errorf("invalid EEEE code length")
	}
	return fmt.Sprintf("00000000000%s00", eeee), nil
}

func getSession(verbose bool) (string, error) {
	headers := map[string]string{
		"Host":       "osrecovery.apple.com",
		"Connection": "close",
		"User-Agent": "InternetRecovery/1.0",
	}

	respHeaders, _, _, err := runQuery("http://osrecovery.apple.com/", headers, nil, false)
	if err != nil {
		return "", err
	}

	if verbose {
		fmt.Println("Session headers:")
		for k, v := range respHeaders {
			fmt.Printf("%s: %s\n", k, v)
		}
	}

	for k, values := range respHeaders {
		if strings.ToLower(k) == "set-cookie" {
			for _, cookie := range values {
				parts := strings.Split(cookie, "; ")
				for _, part := range parts {
					if strings.HasPrefix(part, "session=") {
						return part, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no session in headers")
}

func getImageInfo(session, bid, mlb string, diag bool, osType, cid string) (map[string]string, error) {
	headers := map[string]string{
		"Host":         "osrecovery.apple.com",
		"Connection":   "close",
		"User-Agent":   "InternetRecovery/1.0",
		"Cookie":       session,
		"Content-Type": "text/plain",
	}

	post := map[string]string{
		"cid": generateID(TypeSID, cid),
		"sn":  mlb,
		"bid": bid,
		"k":   generateID(TypeK, ""),
		"fg":  generateID(TypeFG, ""),
	}

	var urlStr string
	if diag {
		urlStr = "http://osrecovery.apple.com/InstallationPayload/Diagnostics"
	} else {
		urlStr = "http://osrecovery.apple.com/InstallationPayload/RecoveryImage"
		post["os"] = osType
	}

	_, output, _, err := runQuery(urlStr, headers, post, false)
	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			info[parts[0]] = parts[1]
		}
	}

	for _, k := range infoRequired {
		if _, ok := info[k]; !ok {
			return nil, fmt.Errorf("missing key %s", k)
		}
	}

	return info, nil
}

func saveImage(urlStr, sess, filename, directory string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	headers := map[string]string{
		"Host":       parsedURL.Hostname(),
		"Connection": "close",
		"User-Agent": "InternetRecovery/1.0",
		"Cookie":     "AssetToken=" + sess,
	}

	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", err
	}

	if filename == "" {
		filename = filepath.Base(parsedURL.Path)
	}
	if strings.Contains(filename, string(os.PathSeparator)) || filename == "" {
		return "", fmt.Errorf("invalid save path %s", filename)
	}

	fullPath := filepath.Join(directory, filename)
	fmt.Printf("Saving %s to %s...\n", urlStr, fullPath)

	_, _, resp, err := runQuery(urlStr, headers, nil, true)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	totalSize := resp.ContentLength
	var size int64
	oldTerminalSize := 0
	buffer := make([]byte, 1024*1024)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			file.Write(buffer[:n])
			size += int64(n)

			terminalSize := getTerminalWidth() - TerminalMargin
			if terminalSize < 0 {
				terminalSize = 0
			}

			if oldTerminalSize != terminalSize {
				fmt.Printf("\r%*s", terminalSize, "")
				oldTerminalSize = terminalSize
			}

			if totalSize > 0 {
				progress := float64(size) / float64(totalSize)
				barWidth := terminalSize / 3
				fmt.Printf("\r%.1f/%.1f MB ", float64(size)/(1024*1024), float64(totalSize)/(1024*1024))
				if terminalSize > 55 {
					filled := int(float64(barWidth) * progress)
					fmt.Printf("|%s%*s|", strings.Repeat("=", filled), barWidth-filled, "")
				}
				fmt.Printf(" %.1f%% downloaded", progress*100)
			} else {
				fmt.Printf("\r%.1f MB downloaded...", float64(size)/(1024*1024))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	fmt.Println("\nDownload complete!")
	return fullPath, nil
}

func getTerminalWidth() int {
	// Try to get terminal width in a cross-platform way
	// Default to 80 if unable to determine
	width := 80

	// This works on Unix-like systems (Linux, macOS)
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		// Terminal detection - use environment variable as fallback
		if cols := os.Getenv("COLUMNS"); cols != "" {
			fmt.Sscanf(cols, "%d", &width)
		}
	}

	return width
}

func verifyChunklist(cnkPath string) ([][2]interface{}, error) {
	file, err := os.Open(cnkPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hashCtx := sha256.New()
	var header ChunkListHeader

	if err := binary.Read(io.TeeReader(file, hashCtx), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if string(header.Magic[:]) != "CNKL" {
		return nil, fmt.Errorf("invalid magic")
	}

	var chunks [][2]interface{}
	for i := uint64(0); i < header.ChunkCount; i++ {
		var chunk Chunk
		if err := binary.Read(io.TeeReader(file, hashCtx), binary.LittleEndian, &chunk); err != nil {
			return nil, err
		}
		chunks = append(chunks, [2]interface{}{chunk.Size, chunk.Hash})
	}

	digest := hashCtx.Sum(nil)

	if header.SignatureMethod == 1 {
		sigBytes := make([]byte, 256)
		if _, err := file.Read(sigBytes); err != nil {
			return nil, err
		}

		signature := new(big.Int).SetBytes(reverseBytes(sigBytes))
		exponent := big.NewInt(0x10001)
		plaintext := new(big.Int).Exp(signature, exponent, AppleEFIROMPublicKey1)

		// Construct expected plaintext with PKCS#1 padding and digest
		expected := new(big.Int)
		expectedStr := "1" + strings.Repeat("f", 404) + "003031300d060960864801650304020105000420" + strings.Repeat("0", 64)
		expected.SetString(expectedStr, 16)
		expected.Or(expected, new(big.Int).SetBytes(digest))

		// Verify signature matches expected plaintext
		if plaintext.Cmp(expected) != 0 {
			return nil, fmt.Errorf("invalid signature")
		}
	} else if header.SignatureMethod == 2 {
		hashBytes := make([]byte, 32)
		if _, err := file.Read(hashBytes); err != nil {
			return nil, err
		}
		if !bytes.Equal(hashBytes, digest) {
			return nil, fmt.Errorf("chunklist missing digital signature")
		}
	}

	return chunks, nil
}

func reverseBytes(b []byte) []byte {
	r := make([]byte, len(b))
	for i := 0; i < len(b); i++ {
		r[i] = b[len(b)-1-i]
	}
	return r
}

func verifyImage(dmgPath, cnkPath string) error {
	fmt.Println("Verifying image with chunklist...")

	chunks, err := verifyChunklist(cnkPath)
	if err != nil {
		return err
	}

	dmgFile, err := os.Open(dmgPath)
	if err != nil {
		return err
	}
	defer dmgFile.Close()

	for i, chunk := range chunks {
		cnkSize := chunk[0].(uint32)
		cnkHash := chunk[1].([32]byte)

		terminalSize := getTerminalWidth() - TerminalMargin
		if terminalSize < 0 {
			terminalSize = 0
		}
		fmt.Printf("\r%-*s", terminalSize, fmt.Sprintf("Chunk %d (%d bytes)", i+1, cnkSize))

		cnkData := make([]byte, cnkSize)
		n, err := dmgFile.Read(cnkData)
		if err != nil && err != io.EOF {
			return err
		}
		if uint32(n) != cnkSize {
			return fmt.Errorf("invalid chunk %d size: expected %d, read %d", i+1, cnkSize, n)
		}

		hash := sha256.Sum256(cnkData)
		if hash != cnkHash {
			return fmt.Errorf("invalid chunk %d: hash mismatch", i+1)
		}
	}

	buf := make([]byte, 1)
	if n, _ := dmgFile.Read(buf); n > 0 {
		return fmt.Errorf("invalid image: larger than chunklist")
	}

	fmt.Println("\nImage verification complete!")
	return nil
}

func actionDownload(boardID, mlb, osType, outdir, basename string, diagnostics, verbose bool) error {
	session, err := getSession(verbose)
	if err != nil {
		return err
	}

	info, err := getImageInfo(session, boardID, mlb, diagnostics, osType, "")
	if err != nil {
		return err
	}

	if verbose {
		fmt.Println(info)
	}

	fmt.Printf("Downloading %s...\n", info[InfoProduct])

	cnkName := basename
	if basename != "" {
		cnkName += ".chunklist"
	}
	cnkPath, err := saveImage(info[InfoSignLink], info[InfoSignSess], cnkName, outdir)
	if err != nil {
		return err
	}

	dmgName := basename
	if basename != "" {
		dmgName += ".dmg"
	}
	dmgPath, err := saveImage(info[InfoImageLink], info[InfoImageSess], dmgName, outdir)
	if err != nil {
		return err
	}

	if err := verifyImage(dmgPath, cnkPath); err != nil {
		fmt.Printf("\rImage verification failed. (%v)\n", err)
		return err
	}

	return nil
}

func actionSelfcheck(verbose bool) error {
	session, err := getSession(verbose)
	if err != nil {
		return err
	}

	validDefault, _ := getImageInfo(session, RecentMac, MLBValid, false, "default", "")
	validLatest, _ := getImageInfo(session, RecentMac, MLBValid, false, "latest", "")
	productDefault, _ := getImageInfo(session, RecentMac, MLBProduct, false, "default", "")
	productLatest, _ := getImageInfo(session, RecentMac, MLBProduct, false, "latest", "")
	genericDefault, _ := getImageInfo(session, RecentMac, MLBZero, false, "default", "")
	genericLatest, _ := getImageInfo(session, RecentMac, MLBZero, false, "latest", "")

	if verbose {
		fmt.Println(validDefault)
		fmt.Println(validLatest)
		fmt.Println(productDefault)
		fmt.Println(productLatest)
		fmt.Println(genericDefault)
		fmt.Println(genericLatest)
	}

	if validDefault[InfoProduct] == validLatest[InfoProduct] {
		return fmt.Errorf("cannot determine any previous product, got %s", validDefault[InfoProduct])
	}

	if productDefault[InfoProduct] != productLatest[InfoProduct] {
		return fmt.Errorf("latest and default do not match for product MLB")
	}

	if genericDefault[InfoProduct] != genericLatest[InfoProduct] {
		return fmt.Errorf("generic MLB gives different product")
	}

	if validLatest[InfoProduct] != genericLatest[InfoProduct] {
		return fmt.Errorf("cannot determine unified latest product")
	}

	if productDefault[InfoProduct] != validDefault[InfoProduct] {
		return fmt.Errorf("valid and product MLB give mismatch")
	}

	fmt.Println("SUCCESS: Found no discrepancies with MLB validation algorithm!")
	return nil
}

func actionVerify(boardID, mlb string, verbose bool) error {
	session, err := getSession(verbose)
	if err != nil {
		return err
	}

	genericLatest, _ := getImageInfo(session, RecentMac, MLBZero, false, "latest", "")
	uvalidDefault, _ := getImageInfo(session, boardID, mlb, false, "default", "")
	uvalidLatest, _ := getImageInfo(session, boardID, mlb, false, "latest", "")
	uproductDefault, _ := getImageInfo(session, boardID, productMLB(mlb), false, "default", "")

	if verbose {
		fmt.Println(genericLatest)
		fmt.Println(uvalidDefault)
		fmt.Println(uvalidLatest)
		fmt.Println(uproductDefault)
	}

	if uvalidDefault[InfoProduct] != uvalidLatest[InfoProduct] {
		if uvalidLatest[InfoProduct] == genericLatest[InfoProduct] {
			fmt.Printf("SUCCESS: %s MLB looks valid and supported!\n", mlb)
		} else {
			fmt.Printf("SUCCESS: %s MLB looks valid, but probably unsupported!\n", mlb)
		}
		return nil
	}

	fmt.Println("UNKNOWN: Run selfcheck, check your board-id, or try again later!")
	return nil
}

func actionGuess(mlb, boardDB string, verbose bool) error {
	anon := strings.HasPrefix(mlb, "000")

	file, err := os.Open(boardDB)
	if err != nil {
		return err
	}
	defer file.Close()

	var db map[string]string
	if err := json.NewDecoder(file).Decode(&db); err != nil {
		return err
	}

	session, err := getSession(verbose)
	if err != nil {
		return err
	}

	genericLatest, _ := getImageInfo(session, RecentMac, MLBZero, false, "latest", "")
	supported := make(map[string][]string)

	for model := range db {
		if anon {
			modelLatest, err := getImageInfo(session, model, MLBZero, false, "latest", "")
			if err != nil {
				continue
			}

			if modelLatest[InfoProduct] != genericLatest[InfoProduct] {
				continue
			}

			userDefault, err := getImageInfo(session, model, mlb, false, "default", "")
			if err != nil {
				continue
			}

			if userDefault[InfoProduct] != genericLatest[InfoProduct] {
				supported[model] = []string{db[model], userDefault[InfoProduct], genericLatest[InfoProduct]}
			}
		} else {
			userLatest, err := getImageInfo(session, model, mlb, false, "latest", "")
			if err != nil {
				continue
			}

			userDefault, err := getImageInfo(session, model, mlb, false, "default", "")
			if err != nil {
				continue
			}

			if userLatest[InfoProduct] != userDefault[InfoProduct] {
				supported[model] = []string{db[model], userDefault[InfoProduct], userLatest[InfoProduct]}
			}
		}
	}

	if len(supported) > 0 {
		fmt.Printf("SUCCESS: MLB %s looks supported for:\n", mlb)
		for model, info := range supported {
			fmt.Printf("- %s, up to %s, default: %s, latest: %s\n", model, info[0], info[1], info[2])
		}
		return nil
	}

	fmt.Printf("UNKNOWN: Failed to determine supported models for MLB %s!\n", mlb)
	return nil
}

func main() {
	action := flag.String("action", "", "Action to perform: download, selfcheck, verify, guess")
	outdir := flag.String("outdir", "com.apple.recovery.boot", "Output directory for downloading")
	basename := flag.String("basename", "", "Base name for downloading")
	boardID := flag.String("board-id", RecentMac, "Board identifier")
	mlb := flag.String("mlb", MLBZero, "Logic board serial")
	code := flag.String("code", "", "Product EEEE code")
	osType := flag.String("os-type", "default", "OS type (default or latest)")
	diagnostics := flag.Bool("diagnostics", false, "Download diagnostics image")
	verbose := flag.Bool("verbose", false, "Print debug information")
	boardDB := flag.String("board-db", "boards.json", "Board list file")

	flag.Parse()

	if *code != "" {
		mlbValue, err := mlbFromEEEE(*code)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		*mlb = mlbValue
	}

	if len(*mlb) != 17 {
		fmt.Fprintln(os.Stderr, "ERROR: Cannot use MLBs in non 17 character format!")
		os.Exit(1)
	}

	var err error
	switch *action {
	case "download":
		err = actionDownload(*boardID, *mlb, *osType, *outdir, *basename, *diagnostics, *verbose)
	case "selfcheck":
		err = actionSelfcheck(*verbose)
	case "verify":
		err = actionVerify(*boardID, *mlb, *verbose)
	case "guess":
		err = actionGuess(*mlb, *boardDB, *verbose)
	default:
		fmt.Fprintln(os.Stderr, "ERROR: Invalid action. Use: download, selfcheck, verify, or guess")
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
