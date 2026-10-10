package gomodule

import (
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type pathAdmissionKind uint8

const (
	pathAdmissionImport pathAdmissionKind = iota
	pathAdmissionModule
)

// Admission scans ASCII path elements without a split slice, registry or size
// quota. Module paths add domain and version spelling constraints; declaration
// and import identities permit local names.
func admitPath(value string, kind pathAdmissionKind) error {
	if value == "" || value[0] == '-' {
		return core.ErrGoModuleContract
	}
	remaining := value
	for {
		element, tail, more := strings.Cut(remaining, "/")
		if !admittedPathElement(element, kind) {
			return core.ErrGoModuleContract
		}
		if !more {
			break
		}
		remaining = tail
	}
	if kind == pathAdmissionModule && (!admittedModuleDomain(value) || !admittedModuleVersion(value)) {
		return core.ErrGoModuleContract
	}
	return nil
}

func admittedPathElement(element string, kind pathAdmissionKind) bool {
	if element == "" || element[len(element)-1] == '.' || (kind == pathAdmissionModule && element[0] == '.') {
		return false
	}
	for index := range len(element) {
		char := element[index]
		if !pathASCIICharacter(char, kind) {
			return false
		}
	}
	stem, _, _ := strings.Cut(element, ".")
	if reservedPathStem(stem) {
		return false
	}
	if tilde := strings.LastIndexByte(stem, '~'); tilde >= 0 && tilde < len(stem)-1 && decimalPathSuffix(stem[tilde+1:]) {
		return false
	}
	return true
}

func pathASCIICharacter(char byte, kind pathAdmissionKind) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' ||
		char == '-' || char == '.' || char == '_' || char == '~' || (kind == pathAdmissionImport && char == '+')
}

func reservedPathStem(stem string) bool {
	if strings.EqualFold(stem, "CON") || strings.EqualFold(stem, "PRN") || strings.EqualFold(stem, "AUX") || strings.EqualFold(stem, "NUL") {
		return true
	}
	return len(stem) == 4 && stem[3] >= '1' && stem[3] <= '9' && (strings.EqualFold(stem[:3], "COM") || strings.EqualFold(stem[:3], "LPT"))
}

func decimalPathSuffix(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func admittedModuleDomain(value string) bool {
	domain, _, _ := strings.Cut(value, "/")
	if !strings.Contains(domain, ".") {
		return false
	}
	for index := range len(domain) {
		char := domain[index]
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' || char == '-') {
			return false
		}
	}
	return true
}

func admittedModuleVersion(value string) bool {
	if strings.HasPrefix(value, "gopkg.in/") {
		return admittedGopkgVersion(value)
	}
	separator := strings.LastIndexByte(value, '/')
	if separator < 0 {
		return true
	}
	last := value[separator+1:]
	if len(last) <= 1 || last[0] != 'v' || !numericVersionSpelling(last[1:]) {
		return true
	}
	number := last[1:]
	return number[0] != '0' && number != "1" && !strings.Contains(number, ".")
}

func numericVersionSpelling(value string) bool {
	for index := range len(value) {
		char := value[index]
		if !(char >= '0' && char <= '9' || char == '.') {
			return false
		}
	}
	return true
}

func admittedGopkgVersion(value string) bool {
	stable := strings.TrimSuffix(value, "-unstable")
	index := len(stable)
	for index > 0 && stable[index-1] >= '0' && stable[index-1] <= '9' {
		index--
	}
	if index == len(stable) || index < 2 || stable[index-2:index] != ".v" {
		return false
	}
	number := stable[index:]
	return number[0] != '0' || (number == "0" && stable == value)
}
