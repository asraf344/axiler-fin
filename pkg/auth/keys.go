/*
 * Copyright (c) 2026 peek8.io
 *
 * Created Date: Tuesday, September 15th 2026, 9:17:40 am
 * Author: Md. Asraful Haque
 *
 */

package auth

import _ "embed"

// RSA private key in PEM format (for demonstration purposes only)
//
//go:embed rsa_private.key
var rsaPrivateKeyPEM string

// RSA public key in PEM format (for demonstration purposes only)
//
//go:embed rsa_public.key
var rsaPublicKeyPEM string

// TODO: In production, load keys from secure storage or environment variables instead of embedding them in the code.
func GetRSAPrivateKeyPEM() string {
	return rsaPrivateKeyPEM
}

// TODO: In production, load keys from secure storage or environment variables instead of embedding them in the code.
func GetRSAPublicKeyPEM() string {
	return rsaPublicKeyPEM
}
