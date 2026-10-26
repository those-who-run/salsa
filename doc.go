// Package salsa is the salsa library; README.md says what it is.
//
// It starts as the template renders it: one placeholder, Name, with its example,
// so the module builds and every check in `make check` (the tests with the
// coverage floor among them) has code to run on from the first commit. The
// library's own API and its tests replace both.
package salsa

// Name returns the module's name. It is the template's placeholder: replace it
// with the library's own API.
func Name() string { return "salsa" }
