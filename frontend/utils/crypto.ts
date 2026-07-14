/**
 * YMessage Client Cryptography Service
 * Implements E2EE using Web Crypto API (ECDH P-256, AES-256-GCM, and HKDF).
 */

// Helper to convert ArrayBuffer to Base64
export function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

// Helper to convert Base64 to ArrayBuffer
export function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64);
  const len = binaryString.length;
  const bytes = new Uint8Array(len);
  for (let i = 0; i < len; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}

export interface KeyPairB64 {
  publicKey: string;
  privateKey: string;
}

/**
 * Generates an ECDH key pair for Identity/Signed/One-Time prekeys
 */
export async function generateKeyPair(): Promise<KeyPairB64> {
  const keyPair = await window.crypto.subtle.generateKey(
    {
      name: 'ECDH',
      namedCurve: 'P-256',
    },
    true, // extractable
    ['deriveKey', 'deriveBits']
  );

  const pubExported = await window.crypto.subtle.exportKey('spki', keyPair.publicKey);
  const privExported = await window.crypto.subtle.exportKey('pkcs8', keyPair.privateKey);

  return {
    publicKey: arrayBufferToBase64(pubExported),
    privateKey: arrayBufferToBase64(privExported),
  };
}

/**
 * Derives a shared symmetric AES-256-GCM key between two users using ECDH
 */
export async function deriveSharedKey(
  myPrivateKeyB64: string,
  theirPublicKeyB64: string
): Promise<CryptoKey> {
  // Import private key
  const privBuffer = base64ToArrayBuffer(myPrivateKeyB64);
  const privateKey = await window.crypto.subtle.importKey(
    'pkcs8',
    privBuffer,
    {
      name: 'ECDH',
      namedCurve: 'P-256',
    },
    false, // not extractable
    ['deriveKey', 'deriveBits']
  );

  // Import public key
  const pubBuffer = base64ToArrayBuffer(theirPublicKeyB64);
  const publicKey = await window.crypto.subtle.importKey(
    'spki',
    pubBuffer,
    {
      name: 'ECDH',
      namedCurve: 'P-256',
    },
    true,
    []
  );

  // Derive shared key
  return await window.crypto.subtle.deriveKey(
    {
      name: 'ECDH',
      public: publicKey,
    },
    privateKey,
    {
      name: 'AES-GCM',
      length: 256,
    },
    true,
    ['encrypt', 'decrypt']
  );
}

/**
 * Encrypts a plaintext string with a derived symmetric key using AES-GCM
 */
export async function encryptMessage(
  sharedKey: CryptoKey,
  plaintext: string
): Promise<{ ciphertext: string; iv: string }> {
  const encoder = new TextEncoder();
  const data = encoder.encode(plaintext);

  // Initialization Vector: 12 bytes is standard for AES-GCM
  const iv = window.crypto.getRandomValues(new Uint8Array(12));

  const encryptedBuffer = await window.crypto.subtle.encrypt(
    {
      name: 'AES-GCM',
      iv: iv,
    },
    sharedKey,
    data
  );

  return {
    ciphertext: arrayBufferToBase64(encryptedBuffer),
    iv: arrayBufferToBase64(iv.buffer),
  };
}

/**
 * Decrypts a ciphertext with a derived symmetric key
 */
export async function decryptMessage(
  sharedKey: CryptoKey,
  ciphertextB64: string,
  ivB64: string
): Promise<string> {
  const encryptedData = base64ToArrayBuffer(ciphertextB64);
  const iv = new Uint8Array(base64ToArrayBuffer(ivB64));

  const decryptedBuffer = await window.crypto.subtle.decrypt(
    {
      name: 'AES-GCM',
      iv: iv,
    },
    sharedKey,
    encryptedData
  );

  const decoder = new TextDecoder();
  return decoder.decode(decryptedBuffer);
}
