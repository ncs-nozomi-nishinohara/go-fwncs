package fwncs

const (
	defaultMemory = 32 << 20 // 32 MB
	Uppercase     = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Lowercase     = "abcdefghijklmnopqrstuvwxyz"
	Alphabetic    = Uppercase + Lowercase
	Numeric       = "0123456789"
	Alphanumeric  = Alphabetic + Numeric
)

type fwNscContext struct{}
