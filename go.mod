module github.com/tlowerison/credential-1password

go 1.16

replace github.com/tlowerison/credential-1password/op => ./op

replace github.com/tlowerison/credential-1password/util => ./util

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/spf13/cobra v1.1.3
	github.com/tidwall/gjson v1.7.4
	github.com/tlowerison/credential-1password/op v0.0.0-00010101000000-000000000000
	github.com/tlowerison/credential-1password/util v0.0.0-00010101000000-000000000000
)
