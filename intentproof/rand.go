package intentproof

import "crypto/rand"

var randReadFn = rand.Read

func randRead(b []byte) (int, error) {
	return randReadFn(b)
}
