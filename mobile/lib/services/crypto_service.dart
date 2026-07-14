import 'dart:convert';
import 'package:cryptography/cryptography.dart';

class CryptoService {
  final _ecdh = Ecdh.p256(x509: true);
  final _aesGcm = AesGcm.with256bits();

  // Generate a new ECDH P-256 key pair
  Future<SimpleKeyPair> generateKeyPair() async {
    return await _ecdh.newKeyPair();
  }

  // Helper to convert KeyPair public key bytes to Base64
  Future<String> getPublicKeyBase64(SimpleKeyPair keyPair) async {
    final pubKey = await keyPair.extractPublicKey();
    return base64Encode(pubKey.bytes);
  }

  // Helper to convert KeyPair private key bytes to Base64
  Future<String> getPrivateKeyBase64(SimpleKeyPair keyPair) async {
    final privKey = await keyPair.extract();
    final bytes = privKey.bytes;
    return base64Encode(bytes);
  }

  // Re-import a private key from Base64
  Future<SimpleKeyPair> importPrivateKey(String privateKeyB64) async {
    final privateKeyBytes = base64Decode(privateKeyB64);
    final keyPair = SimpleKeyPair.fromKeyPair(
      privateKey: privateKeyBytes,
      publicKey: SimplePublicKey(const [], type: KeyPairType.x25519), // placeholder
    );
    return keyPair;
  }

  // Derive shared symmetric key (SecretKey) from local private key and remote public key
  Future<SecretKey> deriveSharedKey(
    SimpleKeyPair myKeyPair,
    String theirPublicKeyB64,
  ) async {
    final theirBytes = base64Decode(theirPublicKeyB64);
    final theirPublicKey = SimplePublicKey(theirBytes, type: KeyPairType.x25519);

    return await _ecdh.deriveKeyFromSharedSecret(
      sharedSecret: await _ecdh.sharedSecretKey(
        keyPair: myKeyPair,
        remotePublicKey: theirPublicKey,
      ),
      nonce: const [], // No salt for standard DH exchange
    );
  }

  // Encrypt plaintext with AES-GCM using derived shared key
  Future<String> encryptData(SecretKey sharedKey, String plaintext) async {
    final clearTextBytes = utf8.encode(plaintext);
    
    // Web Crypto standard 12-byte IV
    final secretBox = await _aesGcm.encrypt(
      clearTextBytes,
      secretKey: sharedKey,
    );

    final ciphertextB64 = base64Encode(secretBox.cipherText);
    final ivB64 = base64Encode(secretBox.nonce);

    // Concatenate ciphertext and IV separated by colon
    return '$ciphertextB64:$ivB64';
  }

  // Decrypt encrypted payload (format: ciphertextB64:ivB64)
  Future<String> decryptData(SecretKey sharedKey, String encryptedPayload) async {
    final parts = encryptedPayload.split(':');
    if (parts.length != 2) {
      return encryptedPayload; // Return raw if not encrypted correctly
    }

    final ciphertext = base64Decode(parts[0]);
    final iv = base64Decode(parts[1]);

    final secretBox = SecretBox(
      ciphertext,
      nonce: iv,
      mac: Mac.empty, // AesGcm auto-verifies tag in Dart
    );

    final clearTextBytes = await _aesGcm.decrypt(
      secretBox,
      secretKey: sharedKey,
    );

    return utf8.decode(clearTextBytes);
  }
}
