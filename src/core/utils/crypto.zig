//! 加密工具模块 - 提供 AES-256-GCM 加解密功能

const std = @import("std");
const crypto = std.crypto;
const Aes256Gcm = std.crypto.aead.aes_gcm.Aes256Gcm;
const HmacSha256 = std.crypto.auth.hmac.sha2.HmacSha256;
const Random = std.Random;

pub const CryptoError = error{
    InvalidKeyLength,
    InvalidNonceLength,
    AuthenticationFailed,
    OutOfMemory,
};

pub const AES256GCM_KEY_SIZE = 32;
pub const AES256GCM_NONCE_SIZE = 12;
pub const AES256GCM_TAG_SIZE = 16;

pub fn generateKey() [AES256GCM_KEY_SIZE]u8 {
    var key: [AES256GCM_KEY_SIZE]u8 = undefined;
    crypto.random.bytes(&key);
    return key;
}

pub fn generateKeyOwned(allocator: std.mem.Allocator) ![]u8 {
    const key = generateKey();
    const owned = try allocator.alloc(u8, key.len);
    @memcpy(owned, &key);
    return owned;
}

pub fn generateNonce() [AES256GCM_NONCE_SIZE]u8 {
    var nonce: [AES256GCM_NONCE_SIZE]u8 = undefined;
    crypto.random.bytes(&nonce);
    return nonce;
}

pub fn generateSalt() [16]u8 {
    var salt: [16]u8 = undefined;
    crypto.random.bytes(&salt);
    return salt;
}

pub fn deriveKey(password: []const u8, salt: [16]u8, output_key: *[AES256GCM_KEY_SIZE]u8) CryptoError!void {
    var key: [AES256GCM_KEY_SIZE]u8 = undefined;
    std.crypto.pbkdf2.pbkdf2(key[0..], password, &salt, 100_000, HmacSha256) catch |err| {
        _ = err;
        return CryptoError.OutOfMemory;
    };
    output_key.* = key;
}

pub fn encrypt(plaintext: []const u8, key: [AES256GCM_KEY_SIZE]u8, nonce: [AES256GCM_NONCE_SIZE]u8, ciphertext: []u8) CryptoError!void {
    if (ciphertext.len < plaintext.len) {
        return CryptoError.InvalidKeyLength;
    }
    var tag: [AES256GCM_TAG_SIZE]u8 = undefined;
    Aes256Gcm.encrypt(ciphertext[0..plaintext.len], &tag, plaintext, &[_]u8{}, nonce, key);
    @memcpy(ciphertext[plaintext.len..plaintext.len + AES256GCM_TAG_SIZE], &tag);
}

pub fn decrypt(ciphertext: []const u8, key: [AES256GCM_KEY_SIZE]u8, nonce: [AES256GCM_NONCE_SIZE]u8, plaintext: []u8) CryptoError!void {
    if (ciphertext.len < plaintext.len + AES256GCM_TAG_SIZE) {
        return CryptoError.InvalidKeyLength;
    }
    if (plaintext.len != ciphertext.len - AES256GCM_TAG_SIZE) {
        return CryptoError.InvalidKeyLength;
    }
    const tag = ciphertext[plaintext.len..plaintext.len + AES256GCM_TAG_SIZE];
    var tag_arr: [AES256GCM_TAG_SIZE]u8 = undefined;
    @memcpy(&tag_arr, tag);
    Aes256Gcm.decrypt(plaintext, ciphertext[0..plaintext.len], tag_arr, &[_]u8{}, nonce, key) catch {
        return CryptoError.AuthenticationFailed;
    };
}

pub fn encryptWithPassword(plaintext: []const u8, password: []const u8, allocator: std.mem.Allocator) ![]u8 {
    const salt = generateSalt();
    var key: [AES256GCM_KEY_SIZE]u8 = undefined;
    deriveKey(password, salt, &key);
    const nonce = generateNonce();
    const ciphertext_len = plaintext.len + AES256GCM_TAG_SIZE;
    const result = try allocator.alloc(u8, salt.len + nonce.len + ciphertext_len);
    @memcpy(result[0..salt.len], &salt);
    @memcpy(result[salt.len..salt.len + nonce.len], &nonce);
    const ct = result[salt.len + nonce.len..];
    var tag: [AES256GCM_TAG_SIZE]u8 = undefined;
    Aes256Gcm.encrypt(ct[0..plaintext.len], &tag, plaintext, &[_]u8{}, nonce, key);
    @memcpy(ct[plaintext.len..plaintext.len + AES256GCM_TAG_SIZE], &tag);
    return result;
}

pub fn decryptWithPassword(ciphertext: []const u8, password: []const u8, allocator: std.mem.Allocator) ![]u8 {
    if (ciphertext.len < 16 + AES256GCM_NONCE_SIZE + AES256GCM_TAG_SIZE) {
        return CryptoError.AuthenticationFailed;
    }
    const salt: *[16]u8 = @ptrCast(@constCast(@alignCast(ciphertext[0..16])));
    const nonce: *[AES256GCM_NONCE_SIZE]u8 = @ptrCast(@constCast(@alignCast(ciphertext[16..16 + AES256GCM_NONCE_SIZE])));
    const ct = ciphertext[16 + AES256GCM_NONCE_SIZE..];
    var key: [AES256GCM_KEY_SIZE]u8 = undefined;
    deriveKey(password, salt.*, &key);
    const plaintext = try allocator.alloc(u8, ct.len - AES256GCM_TAG_SIZE);
    decrypt(ct, key, nonce.*, plaintext) catch {
        allocator.free(plaintext);
        return CryptoError.AuthenticationFailed;
    };
    return plaintext;
}

pub fn generateToken(allocator: std.mem.Allocator) ![]u8 {
    var token: [32]u8 = undefined;
    std.crypto.random.bytes(&token);
    return try allocator.dupe(u8, &token);
}