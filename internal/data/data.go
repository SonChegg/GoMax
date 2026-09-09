// Package data embeds the static assets pymax ships alongside its source:
// the APK fingerprint catalog and the TLS root CA used for the raw TCP
// transport.
package data

import _ "embed"

// ApkFingerprintsJSON is the verbatim contents of pymax's
// src/pymax/_data/apk_fingerprints.json.
//
//go:embed apk_fingerprints.json
var ApkFingerprintsJSON []byte

// RootCACert is the verbatim contents of pymax's
// src/pymax/_data/rootca_ssl_rsa2022.crt, used to validate the Max TCP API
// TLS certificate.
//
//go:embed rootca_ssl_rsa2022.crt
var RootCACert []byte
