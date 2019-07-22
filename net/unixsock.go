package net

type UnixListener struct {
	fd   *netFD
	path string
}
