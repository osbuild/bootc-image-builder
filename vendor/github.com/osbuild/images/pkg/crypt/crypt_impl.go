//go:build !darwin

// Copied from https://github.com/amoghe/go-crypt/blob/b3e291286513a0c993f7c4dd7060d327d2d56143/crypt_r.go
// Original sources are under MIT license:
// The MIT License (MIT)
//
// # Copyright (c) 2015 Akshay Moghe
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// # Package crypt provides wrappers around functions available in crypt.h
//
// It wraps around the GNU specific extension (crypt_r) when it is available
// (i.e. where GOOS=linux). This makes the go function reentrant (and thus
// callable from concurrent goroutines).
package crypt

import (
	"unsafe"
)

/*
	#cgo LDFLAGS: -lcrypt

	// this is needed for Ubuntu
	#define _GNU_SOURCE

	#include <stdlib.h>
	#include <string.h>
	#include <crypt.h>

	char *gnu_ext_crypt(char *pass, char *salt) {
		char *enc = NULL;
		char *ret = NULL;
		struct crypt_data data;
		data.initialized = 0;

		enc = crypt_r(pass, salt, &data);
		if(enc == NULL) {
			return NULL;
		}

		ret = (char *)malloc((strlen(enc)+1) * sizeof(char)); // for trailing null
		strcpy(ret, enc);
		ret[strlen(enc)]= '\0';

	 return ret;
	}
*/
import "C"

// Crypt provides a wrapper around the glibc crypt_r() function.
// For the meaning of the arguments, refer to the package README.
func crypt(pass, salt string) (string, error) {
	c_pass := C.CString(pass)
	defer C.free(unsafe.Pointer(c_pass))

	c_salt := C.CString(salt)
	defer C.free(unsafe.Pointer(c_salt))

	c_enc, err := C.gnu_ext_crypt(c_pass, c_salt)
	if c_enc == nil {
		return "", err
	}
	defer C.free(unsafe.Pointer(c_enc))

	// Return nil error if the string is non-nil.
	// As per the errno.h manpage, functions are allowed to set errno
	// on success. Caller should ignore errno on success.
	return C.GoString(c_enc), nil
}
