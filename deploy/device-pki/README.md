# Device PKI mount

This directory is a mount point only. Never commit private keys.

DEV/PROD secret provisioning places `device-ca.crt` and `device-ca.key` in `DEVICE_PKI_DIR`; compose mounts it read-only at `/run/beefiscal/device-pki`. The CA must be private and dedicated to BeeFiscal SmartDevices. EMQX SSL/WSS listeners must trust the same CA, set `verify_peer` and `fail_if_no_peer_cert=true`, and enforce topic ACLs derived from the certificate device identity.

Authoritative EMQX references: <https://docs.emqx.com/en/emqx/latest/configuration/listener.html> and <https://docs.emqx.com/en/emqx/latest/access-control/authn/x509.html>.
