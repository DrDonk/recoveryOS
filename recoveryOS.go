package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type OSVersion struct {
	Name    string
	BoardID string
}

var osVersions = []OSVersion{
	{"Catalina", "Mac-6F01561E16C75D06"},
	{"Big Sur", "Mac-2BD1B31983FE1663"},
	{"Monterey", "Mac-A5C67F76ED83108C"},
	{"Ventura", "Mac-B4831CEBD52A0C4C"},
	{"Sonoma", "Mac-827FAC58A8FDFA22"},
	{"Sequoia", "Mac-7BA5B2D9E42DDD94"},
	{"Tahoe", "Mac-CFF7D910A743CAAF"},
}

func convert(format, input, output string) error {
	fmt.Printf("Converting to %s:\n", format)
	
	// Check if qemu-img is available
	qemuImg := "qemu-img"
	if runtime.GOOS == "windows" {
		qemuImg = "qemu-img.exe"
	}
	
	if _, err := exec.LookPath(qemuImg); err != nil {
		return fmt.Errorf("qemu-img not found. Please install QEMU first.\n" +
			"Download from: https://www.qemu.org/download/")
	}
	
	args := []string{"convert", "-f", "dmg", "-O", format, input, output, "-p"}
	cmd := exec.Command(qemuImg, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conversion failed: %v", err)
	}
	
	fmt.Printf("Created %s disk: %s\n", format, output)
	return nil
}

func runMacRecovery(boardID, basename string) error {
	fmt.Println("Downloading DMG...\n")
	
	// Determine the macrecovery executable name
	macrecoveryCmd := "./macrecovery"
	if runtime.GOOS == "windows" {
		macrecoveryCmd = "macrecovery.exe"
	}
	
	// Check if macrecovery exists
	if _, err := os.Stat(macrecoveryCmd); os.IsNotExist(err) {
		// Try without ./ prefix
		macrecoveryCmd = "macrecovery"
		if runtime.GOOS == "windows" {
			macrecoveryCmd = "macrecovery.exe"
		}
	}
	
	args := []string{
		"-action=download",
		"-board-id=" + boardID,
		"-mlb=00000000000000000",
		"-basename=" + basename,
		"-outdir=.",
		"-os-type=latest",
	}
	
	cmd := exec.Command(macrecoveryCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("macrecovery failed: %v\nMake sure macrecovery is in the same directory or in your PATH", err)
	}
	
	return nil
}

func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func printBanner() {
	fmt.Println("\nOC4VM recoveryOS Image Maker")
	fmt.Println("=============================")
	fmt.Println("(c) David Parsons 2022-25\n")
}

func selectOS() (string, string, bool) {
	fmt.Println("Create a recoveryOS virtual image")
	for i, os := range osVersions {
		fmt.Printf("%d. %s\n", i+1, os.Name)
	}
	fmt.Println("")
	fmt.Println("0. Exit")
	
	for {
		selection := readInput("Input menu number: ")
		
		if selection == "0" {
			return "", "", false
		}
		
		// Check numeric selections
		for i, os := range osVersions {
			if selection == fmt.Sprintf("%d", i+1) {
				basename := strings.ToLower(strings.ReplaceAll(os.Name, " ", ""))
				return basename, os.BoardID, true
			}
		}
		
		fmt.Println("Invalid selection. Please try again.")
	}
}

func selectConversion(basename string) error {
	dmg := fmt.Sprintf("%s.dmg", basename)
	vmdk := fmt.Sprintf("%s.vmdk", basename)
	qcow2 := fmt.Sprintf("%s.qcow2", basename)
	vhdx := fmt.Sprintf("%s.vhdx", basename)
	raw := fmt.Sprintf("%s.raw", basename)
	
	fmt.Println("\nConvert the recoveryOS virtual image")
	fmt.Println("1. VMware VMDK")
	fmt.Println("2. QEMU QCOW2")
	fmt.Println("3. Microsoft VHDX")
	fmt.Println("4. Raw image")
	fmt.Println("5. All")
	fmt.Println("")
	fmt.Println("0. Exit")
	
	for {
		selection := readInput("Input menu number: ")
		
		switch selection {
		case "0":
			return nil
		case "1":
			return convert("vmdk", dmg, vmdk)
		case "2":
			return convert("qcow2", dmg, qcow2)
		case "3":
			return convert("vhdx", dmg, vhdx)
		case "4":
			return convert("raw", dmg, raw)
		case "5":
			var errors []string
			if err := convert("vmdk", dmg, vmdk); err != nil {
				errors = append(errors, err.Error())
			}
			if err := convert("qcow2", dmg, qcow2); err != nil {
				errors = append(errors, err.Error())
			}
			if err := convert("vhdx", dmg, vhdx); err != nil {
				errors = append(errors, err.Error())
			}
			if err := convert("raw", dmg, raw); err != nil {
				errors = append(errors, err.Error())
			}
			if len(errors) > 0 {
				return fmt.Errorf("some conversions failed:\n%s", strings.Join(errors, "\n"))
			}
			return nil
		default:
			fmt.Println("Invalid selection. Please try again.")
		}
	}
}

func main() {
	printBanner()
	
	// Select OS version
	basename, boardID, ok := selectOS()
	if !ok {
		fmt.Println("Exiting...")
		os.Exit(0)
	}
	
	// Run macrecovery to download
	if err := runMacRecovery(boardID, basename); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	
	// Select conversion format
	if err := selectConversion(basename); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("\nDone! Your recoveryOS image is ready.")
}