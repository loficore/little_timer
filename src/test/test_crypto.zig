const std = @import("std");
const crypto = @import("../core/utils/crypto.zig");

test "AES-256-GCM encrypt/decrypt" {
    const key = crypto.generateKey();
    const nonce = crypto.generateNonce();
    const plaintext = "Hello, World!";
    const ciphertext_len = plaintext.len + crypto.AES256GCM_TAG_SIZE;

    var ciphertext: [100]u8 = undefined;
    try crypto.encrypt(plaintext, key, nonce, ciphertext[0..ciphertext_len]);

    var decrypted: [20]u8 = undefined;
    try crypto.decrypt(ciphertext[0..ciphertext_len], key, nonce, decrypted[0..plaintext.len]);

    try std.testing.expectEqualStrings(plaintext, decrypted[0..plaintext.len]);
}

test "PBKDF2 key derivation" {
    const password = "test_password";
    const salt: [16]u8 = .{ 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15 };
    var key: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
    try crypto.deriveKey(password, salt, &key);
    try std.testing.expect(key.len == crypto.AES256GCM_KEY_SIZE);
}

test "encrypt/decrypt with wrong key fails" {
    const key_a = crypto.generateKey();
    const key_b = crypto.generateKey();
    const nonce = crypto.generateNonce();
    const plaintext = "sensitive data";

    var ciphertext: [100]u8 = undefined;
    try crypto.encrypt(plaintext, key_a, nonce, ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE]);

    var decrypted: [100]u8 = undefined;
    const result = crypto.decrypt(
        ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE],
        key_b,
        nonce,
        decrypted[0..plaintext.len],
    );
    try std.testing.expectError(crypto.CryptoError.AuthenticationFailed, result);
}

test "encrypt/decrypt with wrong nonce fails" {
    const key = crypto.generateKey();
    const nonce_a = crypto.generateNonce();
    const nonce_b = crypto.generateNonce();
    const plaintext = "nonce test data";

    var ciphertext: [100]u8 = undefined;
    try crypto.encrypt(plaintext, key, nonce_a, ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE]);

    var decrypted: [100]u8 = undefined;
    const result = crypto.decrypt(
        ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE],
        key,
        nonce_b,
        decrypted[0..plaintext.len],
    );
    try std.testing.expectError(crypto.CryptoError.AuthenticationFailed, result);
}

test "decrypt corrupted ciphertext fails" {
    const key = crypto.generateKey();
    const nonce = crypto.generateNonce();
    const plaintext = "corruption test";

    var ciphertext: [100]u8 = undefined;
    try crypto.encrypt(plaintext, key, nonce, ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE]);

    ciphertext[0] ^= 0xFF;

    var decrypted: [100]u8 = undefined;
    const result = crypto.decrypt(
        ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE],
        key,
        nonce,
        decrypted[0..plaintext.len],
    );
    try std.testing.expectError(crypto.CryptoError.AuthenticationFailed, result);
}

test "encrypt buffer too small" {
    const key = crypto.generateKey();
    const nonce = crypto.generateNonce();
    const plaintext = "buffer too small test";

    var ciphertext: [5]u8 = undefined;
    const result = crypto.encrypt(plaintext, key, nonce, ciphertext[0..5]);
    try std.testing.expectError(crypto.CryptoError.InvalidKeyLength, result);
}

test "decrypt buffer size mismatch" {
    const key = crypto.generateKey();
    const nonce = crypto.generateNonce();
    const plaintext = "size check";

    var ciphertext: [100]u8 = undefined;
    try crypto.encrypt(plaintext, key, nonce, ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE]);

    var decrypted: [100]u8 = undefined;
    const result = crypto.decrypt(
        ciphertext[0 .. plaintext.len + crypto.AES256GCM_TAG_SIZE],
        key,
        nonce,
        decrypted[0 .. plaintext.len + 5],
    );
    try std.testing.expectError(crypto.CryptoError.InvalidKeyLength, result);
}

test "encryptWithPassword/decryptWithPassword roundtrip" {
    const allocator = std.testing.allocator;
    const plaintext = "password-protected secret";
    const password = "my_secure_password";

    const ciphertext = try crypto.encryptWithPassword(plaintext, password, allocator);
    defer allocator.free(ciphertext);

    const decrypted = try crypto.decryptWithPassword(ciphertext, password, allocator);
    defer allocator.free(decrypted);

    try std.testing.expectEqualStrings(plaintext, decrypted);
}

test "decryptWithPassword wrong password fails" {
    const allocator = std.testing.allocator;
    const plaintext = "wrong password test";
    const password = "correct_password";

    const ciphertext = try crypto.encryptWithPassword(plaintext, password, allocator);
    defer allocator.free(ciphertext);

    const result = crypto.decryptWithPassword(ciphertext, "wrong_password", allocator);
    try std.testing.expectError(crypto.CryptoError.AuthenticationFailed, result);
}

test "decryptWithPassword corrupted data fails" {
    const allocator = std.testing.allocator;
    const plaintext = "corruption test";
    const password = "test_password";

    const ciphertext = try crypto.encryptWithPassword(plaintext, password, allocator);
    defer allocator.free(ciphertext);

    const tiny: [5]u8 = undefined;
    const result = crypto.decryptWithPassword(&tiny, password, allocator);
    try std.testing.expectError(crypto.CryptoError.AuthenticationFailed, result);
}

test "generateKeyOwned returns 32 bytes" {
    const allocator = std.testing.allocator;
    const key = try crypto.generateKeyOwned(allocator);
    defer allocator.free(key);
    try std.testing.expectEqual(@as(usize, 32), key.len);

    var all_zero = true;
    for (key) |b| {
        if (b != 0) { all_zero = false; break; }
    }
    try std.testing.expect(!all_zero);
}

test "generateNonce returns 12 bytes" {
    const nonce = crypto.generateNonce();
    try std.testing.expectEqual(@as(usize, 12), nonce.len);
}

test "generateSalt returns 16 bytes" {
    const salt = crypto.generateSalt();
    try std.testing.expectEqual(@as(usize, 16), salt.len);
}

test "generateToken returns 32-char token" {
    const allocator = std.testing.allocator;
    const token = try crypto.generateToken(allocator);
    defer allocator.free(token);
    try std.testing.expectEqual(@as(usize, 32), token.len);

    const valid_chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
    for (token) |c| {
        try std.testing.expect(std.mem.indexOfScalar(u8, valid_chars, c) != null);
    }
}

test "generateToken produces unique values" {
    const allocator = std.testing.allocator;
    const token_a = try crypto.generateToken(allocator);
    defer allocator.free(token_a);
    const token_b = try crypto.generateToken(allocator);
    defer allocator.free(token_b);
    try std.testing.expect(!std.mem.eql(u8, token_a, token_b));
}

test "deriveKey is deterministic" {
    const password = "deterministic_test";
    const salt: [16]u8 = .{ 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57 };
    var key1: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
    var key2: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
    try crypto.deriveKey(password, salt, &key1);
    try crypto.deriveKey(password, salt, &key2);
    try std.testing.expectEqualStrings(&key1, &key2);
}

test "deriveKey different passwords produce different keys" {
    const salt: [16]u8 = .{ 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25 };
    var key_a: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
    var key_b: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
    try crypto.deriveKey("password_a", salt, &key_a);
    try crypto.deriveKey("password_b", salt, &key_b);
    try std.testing.expect(!std.mem.eql(u8, &key_a, &key_b));
}