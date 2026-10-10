package com.wails.app;

// KeystoreBridge —— Android 硬件密钥托管的 Java 侧实现(Task 7)。
//
// 由 Go 侧 internal/keystore/keystore_android.go 经 JNI 直接调用(方法查找
// 套路与 wails 自身对 WailsBridge facilities 的反射调用一致 —— 普通公开
// 方法,无框架注解;本仓库不存在 @SimpleMethod 注解类型,brief 该措辞所指
// 即"WailsBridge 现状风格"的简单公开方法)。
//
// 设计红线(见 .superpowers/sdd/2026-10-10-hardware-keystore/task-7-brief.md):
//   - 主密钥是 AndroidKeyStore 内 AES/GCM/NoPadding key,
//     alias "little_timer/master_key",non-exportable,
//     密钥材料永不离开硬件、永不跨 JNI;
//   - 不绑生物识别:setUserAuthenticationRequired(false);
//   - SDK>=28 优先 StrongBox(失败仅捕获 KeyStoreException 回退非 StrongBox,
//     老机型/无 StrongBox 硬件照常工作);
//   - seal 产物 blob = iv(12) || ct || tag(16)(Java GCM Cipher.doFinal 的
//     输出即 ct||tag,tag 128 bit),与 Go crypto.Decrypt 的
//     nonce(12)||ct||tag(16) 布局一致;
//   - unseal 失败抛 GeneralSecurityException,消息以 "UNSEAL_FAILED|" 开头
//     (Go mapAndroidError → ErrUnseal);alias 缺失消息以 "KEY_NOT_FOUND|"
//     开头(Go → ErrNotFound);alias 存在但密钥不可用(UnrecoverableKeyException)
//     以 "KEY_UNRECOVERABLE|" 开头(Go → ErrUnavailable,不可误判为 ErrNotFound);
//     ensureKey 生成失败以 "ENSURE_KEY_FAILED|" 开头(Go 侧无独立标记,落入
//     default → ErrUnavailable)。异常消息只含类名与文本,不含密钥材料。

import android.app.Activity;
import android.os.Build;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Log;

import java.io.IOException;
import java.security.GeneralSecurityException;
import java.security.InvalidAlgorithmParameterException;
import java.security.KeyStore;
import java.security.KeyStoreException;
import java.security.SecureRandom;
import java.security.UnrecoverableKeyException;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

/**
 * Hardware-backed AES-GCM seal/unseal via the Android Keystore. All methods
 * are invoked from the Go side over JNI; they are thread-safe (synchronized)
 * because Go goroutines may call them concurrently.
 */
public class KeystoreBridge {
    private static final String TAG = "KeystoreBridge";

    private static final String ANDROID_KEYSTORE = "AndroidKeyStore";
    private static final String ALIAS = "little_timer/master_key";
    private static final String TRANSFORMATION = "AES/GCM/NoPadding";
    private static final int GCM_TAG_BITS = 128;
    private static final int IV_BYTES = 12;

    // Error-message markers consumed by Go mapAndroidError. Keep the literal
    // marker strings in sync with bridgeMark* in
    // neo-src/internal/keystore/keystore_android_core.go:
    //   MARK_KEY_NOT_FOUND     ↔ bridgeMarkKeyNotFound     → ErrNotFound
    //   MARK_UNSEAL_FAILED     ↔ bridgeMarkUnsealFailed    → ErrUnseal
    //   MARK_KEY_UNRECOVERABLE ↔ bridgeMarkKeyUnrecoverable→ ErrUnavailable
    // (改一处需同步另一处)。
    private static final String MARK_KEY_NOT_FOUND = "KEY_NOT_FOUND|";
    private static final String MARK_UNSEAL_FAILED = "UNSEAL_FAILED|";
    private static final String MARK_KEY_UNRECOVERABLE = "KEY_UNRECOVERABLE|";

    private final Activity activity;
    private final SecureRandom random = new SecureRandom();

    public KeystoreBridge(Activity activity) {
        this.activity = activity;
    }

    /**
     * Idempotently create the hardware master key. Never overwrites an
     * existing alias (a regenerated key would orphan every stored blob).
     */
    public synchronized void ensureKey() throws GeneralSecurityException, IOException {
        KeyStore ks = keystore();
        if (ks.containsAlias(ALIAS)) {
            return;
        }
        GeneralSecurityException strongBoxFailure = null;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            // StrongBox first; only the "refused the flag" exceptions
            // (KeyStoreException per brief; AOSP may also throw
            // InvalidAlgorithmParameterException at init on devices without
            // StrongBox hardware) trigger the fallback. Any other failure is
            // fatal and surfaces to Go.
            try {
                generateKey(ks, true);
                Log.i(TAG, "master key generated in StrongBox");
                return;
            } catch (KeyStoreException | InvalidAlgorithmParameterException e) {
                strongBoxFailure = e;
                Log.w(TAG, "StrongBox unavailable, falling back to non-StrongBox key", e);
            }
        }
        try {
            generateKey(ks, false);
            Log.i(TAG, "master key generated (non-StrongBox)");
        } catch (GeneralSecurityException e) {
            if (strongBoxFailure != null) {
                e.addSuppressed(strongBoxFailure);
            }
            throw new GeneralSecurityException("ENSURE_KEY_FAILED|" + describe(e), e);
        }
    }

    /** Report whether the hardware alias exists. */
    public synchronized Boolean hasKey() throws GeneralSecurityException, IOException {
        return keystore().containsAlias(ALIAS);
    }

    /**
     * Encrypt with the hardware key. Returns iv(12) || ct || tag(16):
     * javax.crypto GCM appends the 128-bit tag to doFinal's output.
     */
    public synchronized byte[] seal(byte[] plaintext) throws GeneralSecurityException, IOException {
        if (plaintext == null) {
            throw new IllegalArgumentException("plaintext is null");
        }
        SecretKey key = getKey();
        byte[] iv = new byte[IV_BYTES];
        random.nextBytes(iv);
        Cipher cipher = Cipher.getInstance(TRANSFORMATION);
        cipher.init(Cipher.ENCRYPT_MODE, key, new GCMParameterSpec(GCM_TAG_BITS, iv));
        // Empty plaintext is legal in GCM (empty ct) and yields a 28-byte blob,
        // matching the Go-side minimum in splitAndroidBlob.
        byte[] ctTag = cipher.doFinal(plaintext);
        byte[] blob = new byte[iv.length + ctTag.length];
        System.arraycopy(iv, 0, blob, 0, iv.length);
        System.arraycopy(ctTag, 0, blob, iv.length, ctTag.length);
        return blob;
    }

    /**
     * Decrypt a seal blob. GCM authentication failures surface as
     * GeneralSecurityException("UNSEAL_FAILED|...") which Go maps to ErrUnseal.
     */
    public synchronized byte[] unseal(byte[] blob) throws GeneralSecurityException, IOException {
        if (blob == null || blob.length < IV_BYTES + GCM_TAG_BITS / 8) {
            throw new GeneralSecurityException(MARK_UNSEAL_FAILED + "blob shorter than iv(12)+tag(16)");
        }
        SecretKey key = getKey();
        Cipher cipher = Cipher.getInstance(TRANSFORMATION);
        cipher.init(Cipher.DECRYPT_MODE, key,
                new GCMParameterSpec(GCM_TAG_BITS, blob, 0, IV_BYTES));
        try {
            return cipher.doFinal(blob, IV_BYTES, blob.length - IV_BYTES);
        } catch (GeneralSecurityException e) {
            // BadTag / invalid key material: never leak details beyond marker.
            throw new GeneralSecurityException(MARK_UNSEAL_FAILED + describe(e), e);
        }
    }

    /** Remove the hardware alias. A missing alias is a no-op (idempotent). */
    public synchronized void deleteKey() throws GeneralSecurityException, IOException {
        KeyStore ks = keystore();
        if (ks.containsAlias(ALIAS)) {
            ks.deleteEntry(ALIAS);
            Log.i(TAG, "master key alias deleted");
        }
    }

    // MARK: - internals

    private KeyStore keystore() throws GeneralSecurityException, IOException {
        KeyStore ks = KeyStore.getInstance(ANDROID_KEYSTORE);
        ks.load(null);
        return ks;
    }

    private SecretKey getKey() throws GeneralSecurityException, IOException {
        KeyStore ks = keystore();
        SecretKey key;
        try {
            key = (SecretKey) ks.getKey(ALIAS, null);
        } catch (UnrecoverableKeyException e) {
            // 别名存在但密钥不可用(OTA/恢复出厂后 Keystore 污染)。绝不复用
            // KEY_NOT_FOUND:那会让 Go 侧 hasKey()==true 时 EnsureMasterKey 短路
            // 成功、Seal 却永远 ErrNotFound 而无恢复路径(死锁)。改用独立标记
            // KEY_UNRECOVERABLE,Go mapAndroidError 映射为 ErrUnavailable。
            throw new GeneralSecurityException(MARK_KEY_UNRECOVERABLE + describe(e), e);
        }
        if (key == null) {
            throw new GeneralSecurityException(MARK_KEY_NOT_FOUND + " alias " + ALIAS + " missing");
        }
        return key;
    }

    private void generateKey(KeyStore ks, boolean strongBox) throws GeneralSecurityException {
        KeyGenerator kg = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE);
        KeyGenParameterSpec.Builder builder =
                new KeyGenParameterSpec.Builder(ALIAS,
                        KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                        .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                        .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                        .setKeySize(256)
                        // 红线:不绑生物识别/设备凭证(硬件密钥保护 ≠ 用户在场证明)。
                        .setUserAuthenticationRequired(false);
        // AndroidKeyStore 的 app-generated key 本身就 non-exportable
        // (getKey 返回只可使用的代理对象);无需也不应依赖 API 30 的
        // setNonExportableParams() 变体,minSdk 21 一条路径全覆盖。
        if (strongBox && Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            builder.setIsStrongBoxBacked(true);
        }
        kg.init(builder.build());
        kg.generateKey();
        if (!ks.containsAlias(ALIAS)) {
            throw new KeyStoreException("keygen completed but alias " + ALIAS + " absent");
        }
    }

    private static String describe(Throwable t) {
        String m = t.getMessage();
        return t.getClass().getSimpleName() + (m != null ? ": " + m : "");
    }
}
