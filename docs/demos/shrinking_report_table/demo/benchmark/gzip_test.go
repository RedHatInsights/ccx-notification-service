package main_test

import (
	"bytes"
	"compress/gzip"
	"os"

	"testing"
)

var (
	text1 string = ""
	text2 string
	text3 string
)

func init() {
	content, err := os.ReadFile("../data/empty_report.json")
	if err != nil {
		panic(err)
	}

	text2 = string(content)

	content, err = os.ReadFile("../data/large_report.json")
	if err != nil {
		panic(err)
	}

	text3 = string(content)
}

func gzipBytes(src *[]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(*src)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gzipString(src *string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write([]byte(*src))
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func BenchmarkGzipEmptyString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text1)
	}
}

func BenchmarkGzipSimpleJSONAsString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text2)
	}
}

func BenchmarkGzipRuleReportAsString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gzipString(&text3)
	}
}

func BenchmarkGzipZeroBytes(b *testing.B) {
	bytes1 := []byte(text1)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes1)
	}
}

func BenchmarkGzipSimpleJSONAsBytes(b *testing.B) {
	bytes2 := []byte(text2)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes2)
	}
}

func BenchmarkGzipRuleReportAsBytes(b *testing.B) {
	bytes3 := []byte(text3)

	for i := 0; i < b.N; i++ {
		gzipBytes(&bytes3)
	}
}
