module example.com/app

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	golang.org/x/text v0.25.0 // indirect
)

replace github.com/google/uuid => ../uuid
