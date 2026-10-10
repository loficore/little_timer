//go:build android

// keystore_android.go —— android-keystore 后端的 JNI 传输薄适配器。
//
// 全部业务逻辑(错误映射、blob 校验、Keystore 面)在无 tag 的
// keystore_android_core.go;本文件只做两件事:
//  1. 导出 `Java_com_wails_app_WailsBridge_nativeKeystoreInit`,由
//     WailsBridge.initialize() 把 com.wails.app.KeystoreBridge 实例注册
//     进来(全局引用 + JavaVM),与 wails 自身 nativeInit 的桥接套路一致;
//  2. 以 cgo JNI 调用 KeystoreBridge 的 ensureKey/hasKey/seal/unseal/
//     deleteKey 五个方法,Java 异常经 C 层转成 "CLASS|message" 字符串,
//     由 core 的 mapAndroidError 映射到哨兵错误。
//
// 主密钥只存在于硬件 Keystore 内,从不跨 JNI 明文往返;跨 JNI 的只有
// Seal/Unseal 的入参与产物 blob(iv(12)||ct||tag)。异常消息只含
// Java 类名与消息文本,不含密钥材料。

package keystore

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static JavaVM* g_lsJvm = NULL;
static jobject g_lsBridge = NULL; // KeystoreBridge 全局引用

// 由 Java_com_wails_app_WailsBridge_nativeKeystoreInit 调用。
static void lsStoreBridge(JNIEnv* env, jobject bridge) {
    if ((*env)->GetJavaVM(env, &g_lsJvm) != 0) {
        g_lsJvm = NULL;
        return;
    }
    if (g_lsBridge != NULL) {
        (*env)->DeleteGlobalRef(env, g_lsBridge);
        g_lsBridge = NULL;
    }
    if (bridge != NULL) {
        g_lsBridge = (*env)->NewGlobalRef(env, bridge);
    }
    if (g_lsBridge == NULL) {
        g_lsJvm = NULL;
    }
}

static int lsReady(void) {
    return g_lsJvm != NULL && g_lsBridge != NULL ? 1 : 0;
}

// 取当前线程 JNIEnv(必要时 Attach),*needsDetach 指示调用结束需否 Detach。
static JNIEnv* lsEnv(int* needsDetach) {
    *needsDetach = 0;
    if (g_lsJvm == NULL) return NULL;
    JNIEnv* env = NULL;
    jint r = (*g_lsJvm)->GetEnv(g_lsJvm, (void**)&env, JNI_VERSION_1_6);
    if (r == JNI_EDETACHED) {
        if ((*g_lsJvm)->AttachCurrentThread(g_lsJvm, &env, NULL) != 0) return NULL;
        *needsDetach = 1;
    } else if (r != JNI_OK) {
        return NULL;
    }
    return env;
}

static void lsReleaseEnv(int needsDetach) {
    if (needsDetach && g_lsJvm != NULL) {
        (*g_lsJvm)->DetachCurrentThread(g_lsJvm);
    }
}

// 捕获并清除挂起异常,返回 malloc 的 "CLASS|message"(绝不为 NULL)。
// 消息文本由 Java 侧以 KEY_NOT_FOUND/UNSEAL_FAILED 标记开头约定。
static char* lsDescribeException(JNIEnv* env) {
    jthrowable thrown = (*env)->ExceptionOccurred(env);
    if (thrown == NULL) {
        return strdup("UNKNOWN|unknown bridge failure");
    }
    (*env)->ExceptionClear(env);

    char nameBuf[256];
    snprintf(nameBuf, sizeof(nameBuf), "%s", "UNKNOWN");
    char* msgBuf = NULL;
    jstring jName = NULL, jMsg = NULL;
    jclass thClass = (*env)->GetObjectClass(env, thrown);
    if (thClass != NULL) {
        // 类名须经 java.lang.Class#getName() 取得:receiver 是 thClass
        // 本身,所以要在它的类(GetObjectClass(thClass) 即 java.lang.Class)
        // 上查方法 ID;在 thClass 上查 "getName" 会 NoSuchMethodError。
        jclass classCls = (*env)->GetObjectClass(env, thClass);
        if (classCls != NULL) {
            jmethodID getNameMid = (*env)->GetMethodID(env, classCls, "getName", "()Ljava/lang/String;");
            if (getNameMid != NULL) {
                jName = (jstring)(*env)->CallObjectMethod(env, thClass, getNameMid);
                if (jName != NULL) {
                    const char* c = (*env)->GetStringUTFChars(env, jName, NULL);
                    if (c != NULL) {
                        snprintf(nameBuf, sizeof(nameBuf), "%s", c);
                        (*env)->ReleaseStringUTFChars(env, jName, c);
                    }
                }
            }
            if ((*env)->ExceptionCheck(env)) {
                (*env)->ExceptionClear(env);
            }
            (*env)->DeleteLocalRef(env, classCls);
        }
        // Throwable#getMessage 定义在 Throwable 上,异常子类可继承查到。
        jmethodID getMsgMid = (*env)->GetMethodID(env, thClass, "getMessage", "()Ljava/lang/String;");
        if (getMsgMid != NULL) {
            jMsg = (jstring)(*env)->CallObjectMethod(env, thrown, getMsgMid);
            if (jMsg != NULL) {
                const char* c = (*env)->GetStringUTFChars(env, jMsg, NULL);
                if (c != NULL) {
                    msgBuf = strdup(c);
                    (*env)->ReleaseStringUTFChars(env, jMsg, c);
                }
            }
        }
        if ((*env)->ExceptionCheck(env)) {
            (*env)->ExceptionClear(env);
        }
        (*env)->DeleteLocalRef(env, thClass);
    }
    if (jName != NULL) (*env)->DeleteLocalRef(env, jName);
    if (jMsg != NULL) (*env)->DeleteLocalRef(env, jMsg);
    (*env)->DeleteLocalRef(env, thrown);

    const char* msg = msgBuf != NULL ? msgBuf : "no message";
    size_t need = strlen(nameBuf) + strlen(msg) + 2;
    char* out = (char*)malloc(need);
    if (out != NULL) {
        snprintf(out, need, "%s|%s", nameBuf, msg);
    } else {
        out = strdup("UNKNOWN|oom");
    }
    free(msgBuf);
    return out;
}

// 查方法 ID;失败(NoSuchMethodError)时返回非 NULL 的错误串。
static jmethodID lsMethodId(JNIEnv* env, jclass bridgeClass, const char* name, const char* sig, char** errmsg) {
    jmethodID mid = (*env)->GetMethodID(env, bridgeClass, name, sig);
    if (mid == NULL) {
        char* desc = lsDescribeException(env);
        size_t need = strlen(desc) + strlen(name) + 3;
        char* out = (char*)malloc(need);
        if (out != NULL) {
            snprintf(out, need, "%s @%s", desc, name);
        }
        free(desc);
        *errmsg = out != NULL ? out : strdup("UNKNOWN|method lookup failed");
        return NULL;
    }
    *errmsg = NULL;
    return mid;
}

// 无参 void 调用:ensureKey/deleteKey。成功返回 NULL。
static char* lsVoidCall(const char* name) {
    if (!lsReady()) return strdup("BRIDGE_DISCONNECTED|keystore bridge not registered");
    int detach = 0;
    JNIEnv* env = lsEnv(&detach);
    if (env == NULL) return strdup("BRIDGE_DISCONNECTED|JVM unavailable");
    char* err = NULL;
    jclass cls = (*env)->GetObjectClass(env, g_lsBridge);
    if (cls == NULL) {
        lsReleaseEnv(detach);
        return strdup("UNKNOWN|bridge class unavailable");
    }
    char* lookupErr = NULL;
    jmethodID mid = lsMethodId(env, cls, name, "()V", &lookupErr);
    if (mid != NULL) {
        (*env)->CallVoidMethod(env, g_lsBridge, mid);
        if ((*env)->ExceptionCheck(env)) err = lsDescribeException(env);
    } else {
        err = lookupErr;
    }
    (*env)->DeleteLocalRef(env, cls);
    lsReleaseEnv(detach);
    return err;
}

// hasKey():Boolean —— 结果写 *outVal(1/0)。成功返回 NULL。
static char* lsHasKey(int* outVal) {
    if (!lsReady()) return strdup("BRIDGE_DISCONNECTED|keystore bridge not registered");
    int detach = 0;
    JNIEnv* env = lsEnv(&detach);
    if (env == NULL) return strdup("BRIDGE_DISCONNECTED|JVM unavailable");
    char* err = NULL;
    jclass cls = (*env)->GetObjectClass(env, g_lsBridge);
    if (cls == NULL) {
        lsReleaseEnv(detach);
        return strdup("UNKNOWN|bridge class unavailable");
    }
    char* lookupErr = NULL;
    jmethodID mid = lsMethodId(env, cls, "hasKey", "()Ljava/lang/Boolean;", &lookupErr);
    if (mid != NULL) {
        jobject jv = (*env)->CallObjectMethod(env, g_lsBridge, mid);
        if ((*env)->ExceptionCheck(env)) {
            err = lsDescribeException(env);
        } else if (jv == NULL) {
            err = strdup("UNKNOWN|hasKey returned null");
        } else {
            jclass boolCls = (*env)->GetObjectClass(env, jv);
            jmethodID valMid = (*env)->GetMethodID(env, boolCls, "booleanValue", "()Z");
            if (valMid == NULL) {
                err = lsDescribeException(env);
            } else {
                *outVal = (*env)->CallBooleanMethod(env, jv, valMid) == JNI_TRUE ? 1 : 0;
                if ((*env)->ExceptionCheck(env)) err = lsDescribeException(env);
            }
            (*env)->DeleteLocalRef(env, boolCls);
            (*env)->DeleteLocalRef(env, jv);
        }
    } else {
        err = lookupErr;
    }
    (*env)->DeleteLocalRef(env, cls);
    lsReleaseEnv(detach);
    return err;
}

// seal/unseal:byte[] → byte[]。成功时 *out 为 malloc 缓冲(*outLen 字节),
// 调用方 free;失败返回错误串且不写 *out。
static char* lsBytesCall(const char* name, const void* in, int inLen, void** out, int* outLen) {
    *out = NULL;
    *outLen = 0;
    if (!lsReady()) return strdup("BRIDGE_DISCONNECTED|keystore bridge not registered");
    int detach = 0;
    JNIEnv* env = lsEnv(&detach);
    if (env == NULL) return strdup("BRIDGE_DISCONNECTED|JVM unavailable");
    char* err = NULL;
    jclass cls = (*env)->GetObjectClass(env, g_lsBridge);
    if (cls == NULL) {
        lsReleaseEnv(detach);
        return strdup("UNKNOWN|bridge class unavailable");
    }
    char* lookupErr = NULL;
    jmethodID mid = lsMethodId(env, cls, name, "([B)[B", &lookupErr);
    if (mid != NULL) {
        jbyteArray arg = (*env)->NewByteArray(env, inLen < 0 ? 0 : inLen);
        if (arg == NULL) {
            err = lsDescribeException(env);
        } else {
            if (in != NULL && inLen > 0) {
                (*env)->SetByteArrayRegion(env, arg, 0, inLen, (const jbyte*)in);
            }
            jbyteArray res = (jbyteArray)(*env)->CallObjectMethod(env, g_lsBridge, mid, arg);
            if ((*env)->ExceptionCheck(env)) {
                err = lsDescribeException(env);
            } else if (res == NULL) {
                err = strdup("UNKNOWN|bridge returned null");
            } else {
                jint n = (*env)->GetArrayLength(env, res);
                jbyte* data = (*env)->GetByteArrayElements(env, res, NULL);
                if (data == NULL) {
                    err = lsDescribeException(env);
                } else {
                    void* copy = malloc(n > 0 ? (size_t)n : 1);
                    if (copy == NULL) {
                        err = strdup("UNKNOWN|oom copying result");
                    } else {
                        memcpy(copy, data, (size_t)n);
                        *out = copy;
                        *outLen = (int)n;
                    }
                    (*env)->ReleaseByteArrayElements(env, res, data, JNI_ABORT);
                }
            }
            (*env)->DeleteLocalRef(env, res);
            (*env)->DeleteLocalRef(env, arg);
        }
    } else {
        err = lookupErr;
    }
    (*env)->DeleteLocalRef(env, cls);
    lsReleaseEnv(detach);
    return err;
}
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

// jniBridge 是 androidBridge 的生产传输:cgo → com.wails.app.KeystoreBridge。
type jniBridge struct{}

func (jniBridge) reachable() bool { return C.lsReady() == 1 }

func (jniBridge) ensureKey() error { return lsVoidCallByName("ensureKey") }

func (jniBridge) deleteKey() error { return lsVoidCallByName("deleteKey") }

// lsVoidCallByName 负责方法名的 CString 编解码与释放。
func lsVoidCallByName(name string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return wrapBridgeError(C.lsVoidCall(cName))
}

func (jniBridge) hasKey() (bool, error) {
	var v C.int
	if err := wrapBridgeError(C.lsHasKey(&v)); err != nil {
		return false, err
	}
	return v == 1, nil
}

func (jniBridge) seal(plaintext []byte) ([]byte, error) {
	return bridgeBytesCall("seal", plaintext)
}

func (jniBridge) unseal(blob []byte) ([]byte, error) {
	return bridgeBytesCall("unseal", blob)
}

// newAndroidKeystore 返回接好 JNI 传输的 android-keystore 后端
// (brief 约定的构造面)。
func newAndroidKeystore() Keystore { return newAndroidKeystoreWithBridge(jniBridge{}) }

// init 把本后端注册给 Current()(机制与 keystore_linux.go 一致;
// keystore_linux.go 已用 `linux && !android` 避开 GOOS=android 下的冲突)。
func init() { currentPlatform = newAndroidKeystore() }

// wrapBridgeError 释放 C 侧错误串并转成 Go error;NULL 表示成功。
func wrapBridgeError(cmsg *C.char) error {
	if cmsg == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cmsg))
	msg := C.GoString(cmsg)
	return errors.New("android keystore bridge: " + msg)
}

// bridgeBytesCall 经 JNI 调 seal/unseal,[]byte 双向原样往返。
func bridgeBytesCall(name string, in []byte) ([]byte, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var cIn unsafe.Pointer
	if len(in) > 0 {
		cIn = C.CBytes(in)
		defer C.free(cIn)
	}
	runtime.KeepAlive(in)

	var cOut unsafe.Pointer
	var cLen C.int
	cmsg := C.lsBytesCall(cName, cIn, C.int(len(in)), &cOut, &cLen)
	if err := wrapBridgeError(cmsg); err != nil {
		return nil, err
	}
	if cOut == nil {
		return nil, errors.New("android keystore bridge: UNKNOWN|null result")
	}
	defer C.free(cOut)
	return C.GoBytes(cOut, cLen), nil
}

// Java_com_wails_app_WailsBridge_nativeKeystoreInit 由
// WailsBridge.initialize() 调用,注册 KeystoreBridge 实例(与 wails 自身
// nativeInit 的符号命名/导出套路一致,见 wails
// pkg/application/application_android.go:681)。
//
//export Java_com_wails_app_WailsBridge_nativeKeystoreInit
func Java_com_wails_app_WailsBridge_nativeKeystoreInit(env *C.JNIEnv, _ C.jclass, bridge C.jobject) {
	C.lsStoreBridge(env, bridge)
}
