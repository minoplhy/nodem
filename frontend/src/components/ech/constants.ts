export const HPKE_CIPHER_SUITE_PRESETS = [
  {
    value: 'x25519,hkdf-sha256,aes-128-gcm',
    label: 'X25519 / HKDF-SHA256 / AES-128-GCM (Standard Recommended)',
  },
  {
    value: 'x25519,hkdf-sha256,chacha20-poly1305',
    label: 'X25519 / HKDF-SHA256 / ChaCha20-Poly1305 (ARM / Mobile Optimized)',
  },
  {
    value: 'x25519,hkdf-sha256,aes-256-gcm',
    label: 'X25519 / HKDF-SHA256 / AES-256-GCM (High Security)',
  },
  {
    value: 'p256,hkdf-sha256,aes-128-gcm',
    label: 'NIST P-256 / HKDF-SHA256 / AES-128-GCM (NIST Standard)',
  },
  {
    value: 'p384,hkdf-sha384,aes-256-gcm',
    label: 'NIST P-384 / HKDF-SHA384 / AES-256-GCM (NIST High Security)',
  },
  {
    value: 'custom',
    label: 'Custom HPKE Suite...',
  },
];
