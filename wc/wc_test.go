package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// file names
const (
	file1 = "test_files/lang_ru.txt"                 // foreign language
	file2 = "test_files/dict_en.txt"                 // US dict words
	file3 = "test_files/spaces_en.txt"               // \s, \t, \v, etc.
	file4 = "test_files/coreutils_man_en.txt"        // so meta
	file5 = "--files0-from=test_files/file_list.txt" // --files0-from=...
)

// file outputs with newlines
const (
	file1_res = "  196  4500 29847 54000   498 test_files/lang_ru.txt\n"
	file2_res = " 500 1000 9719 9722   38 test_files/dict_en.txt\n"
	file3_res = "  9  26 128 128  51 test_files/spaces_en.txt\n"
	file4_res = "  39  479 2824 2876  730 test_files/coreutils_man_en.txt\n"
	// stripped off the `./` from file names...
	file5_res = `    0     1   138   138   133 ./test_files/file_list.txt
    9    26   128   128    51 ./test_files/spaces_en.txt
  500  1000  9719  9722    38 ./test_files/dict_en.txt
  196  4500 29847 54000   498 ./test_files/lang_ru.txt
   39   479  2824  2876   730 ./test_files/coreutils_man_en.txt
  744  6006 42656 66864   730 total
`
)

// foreign language test
func TestFile1(t *testing.T) {
	cmd := exec.Command("wc", "-lwmcL", file1)
	b, err := cmd.Output()
	s := string(b)
	if err != nil {
		t.Error(err)
	}
	if s != file1_res {
		fmt.Printf("Expected:\n%s\n\nGot:\n%s", file1_res, s)
		t.Error("Test 1 failed")
	}
}

// US dictionary words test
func TestFile2(t *testing.T) {
	cmd := exec.Command("wc", "-lwmcL", file2)
	b, err := cmd.Output()
	s := string(b)
	if err != nil {
		t.Error(err)
	}
	if s != file2_res {
		fmt.Printf("Expected:\n%s\n\nGot:\n%s", file2_res, s)
		t.Error("Test 2 failed")
	}
}

// \t, \s, \v, etc test (spaces test)
func TestFile3(t *testing.T) {
	cmd := exec.Command("wc", "-lwmcL", file3)
	b, err := cmd.Output()
	s := string(b)
	if err != nil {
		t.Error(err)
	}
	if s != file3_res {
		fmt.Printf("Expected:\n%s\n\nGot:\n%s", file3_res, s)
		t.Error("Test 3 failed")
	}
}

// meta test :)
func TestFile4(t *testing.T) {
	cmd := exec.Command("wc", "-lwmcL", file4)
	b, err := cmd.Output()
	s := string(b)
	if err != nil {
		t.Error(err)
	}
	if s != file4_res {
		fmt.Printf("Expected:\n%s\n\nGot:\n%s", file4_res, s)
		t.Error("Test 4 failed")
	}
}

// --files0-from= test
func TestFile5(t *testing.T) {
	cmd := exec.Command("wc", "-lwmcL", file5)
	b, err := cmd.Output()
	s := string(b)
	if err != nil {
		t.Error(err)
	}
	if s != file5_res {
		fmt.Printf("Expected:\n%s\n\nGot:\n%s", file5_res, s)
		t.Error("Test 5 failed")
	}
}
