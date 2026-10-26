package salsa_test

import (
	"errors"
	"fmt"
	"os"

	"github.com/those-who-run/salsa"
)

func ExampleDecrypt() {
	// A datagram as the PS5 sends it to TelemetryPort; this one is synthetic.
	datagram, err := os.ReadFile("testdata/golden/b.datagram")
	if err != nil {
		panic(err)
	}
	plain, err := salsa.Decrypt(datagram)
	if err != nil {
		panic(err)
	}
	fmt.Println(plain.Kind, len(plain.Bytes), string(plain.Bytes[:4]))

	// 368 bytes is the unconfirmed "C" packet, not one Decrypt knows.
	_, err = salsa.Decrypt(make([]byte, 368))
	fmt.Println(errors.Is(err, salsa.ErrLength))
	// Output:
	// B 316 0S7G
	// true
}
