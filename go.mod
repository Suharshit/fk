module github.com/Suharshit/fk

go 1.23.0

toolchain go1.24.7

replace golang.org/x/term => github.com/golang/term v0.30.0

replace golang.org/x/sys => github.com/golang/sys v0.31.0

replace golang.org/x/image => github.com/golang/image v0.25.0

require (
	github.com/creack/pty v1.1.24
	github.com/gorilla/websocket v1.5.3
	golang.org/x/image v0.25.0
	golang.org/x/term v0.30.0
)

require golang.org/x/sys v0.31.0 // indirect
