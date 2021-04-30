module github.com/tlowerison/credential-1password

go 1.16

replace (
  github.com/tlowerison/credential-1password/keystore => ./keystore
  github.com/tlowerison/credential-1password/op => ./op
  github.com/tlowerison/credential-1password/util => ./util
)

require (
	github.com/spf13/cobra v1.1.3
	github.com/tidwall/gjson v1.7.4
	github.com/tlowerison/credential-1password/keystore v0.0.0-00010101000000-000000000000 // indirect
	github.com/tlowerison/credential-1password/op v0.0.0-00010101000000-000000000000
	github.com/tlowerison/credential-1password/util v0.0.0-00010101000000-000000000000
)
