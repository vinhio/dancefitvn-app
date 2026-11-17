package qr

import "testing"

// BenchmarkEncode is informational: encoding runs once per pairing code.
func BenchmarkEncode(b *testing.B) {
	data := []byte(pairingURL)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(data, LevelM); err != nil {
			b.Fatal(err)
		}
	}
}
