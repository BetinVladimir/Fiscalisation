# BeeFiscal device manufacturing

Guarded station CLI for ESP32-S3 release hash verification, flashing and OIDC
workload-authenticated registration. The token is read only from
`BEEFISCAL_FACTORY_OIDC_TOKEN`; it is never accepted as a command-line argument.

```bash
python -m pip install -e .
beefiscal-factory flash --port /dev/ttyUSB0 --firmware firmware.bin --expected-sha256 <sha256>
beefiscal-factory register --backend https://fiscal.example --evidence evidence.json --proof <device-proof>
```

`device-proof` is produced by the device itself: unpadded base64url of the
64-byte IEEE P1363 ECDSA P-256 signature (`r || s`) over SHA-256 of the
canonical manufacturing proof. The station must never generate, receive or
persist the device private key.

The production station must obtain the OIDC token from workload identity and
must verify a signed release manifest before invoking `flash`.
