module github.com/tlowerison/credential-1password/util/test

go 1.16

replace github.com/tlowerison/credential-1password/keystore => ../../keystore/

replace github.com/tlowerison/credential-1password/op => ../../op/

replace github.com/tlowerison/credential-1password/util => ../

require (
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/tlowerison/credential-1password/keystore v0.0.0-00010101000000-000000000000 // indirect
	github.com/tlowerison/credential-1password/util v0.0.0-00010101000000-000000000000
)
