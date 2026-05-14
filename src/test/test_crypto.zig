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
    crypto.deriveKey(password, salt, &key);
    try std.testing.expect(key.len == crypto.AES256GCM_KEY_SIZE);
}