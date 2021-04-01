module github.com/tlowerison/credential-1password/util

go 1.16

replace github.com/tlowerison/credential-1password/op => ../op

require (
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	github.com/tidwall/gjson v1.7.4
	github.com/tlowerison/credential-1password/op v0.0.0-00010101000000-000000000000
)
