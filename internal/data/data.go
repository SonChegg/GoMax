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

// SubCACert is the "Russian Trusted Sub CA" certificate, extracted from the
// official Android client's trust-store builder (defpackage/kb7.java) and
// verified to be signed by RootCACert. The Android client layers both
// certs onto the system trust store; pymax only ships the root, but real
// server chains may terminate at the sub CA, so both are needed to match
// the official client's trust behavior.
//
//go:embed subca_ssl_rsa2022.crt
var SubCACert []byte
