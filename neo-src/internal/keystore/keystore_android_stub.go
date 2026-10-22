//go:build !android

// keystore_android_stub.go —— 非 android 构建的编译期占位。
//
// 存在的意义(brief Step 文件清单约定):
//   - android-keystore 后端的可绑定表面(newAndroidKeystoreWithBridge、
//     androidBridge、错误映射与 blob 校验)全部位于无 tag 的
//     keystore_android_core.go,任何平台都能编译;本文件只做一条编译期
//     断言,保证元数据生成流程(gomobile/wails3 bindings 在主机侧静态分析
//     本包)即使不带 `android` tag 也能解析到完整的 android 后端符号。
//   - currentPlatform 的注册仍由各平台实现文件负责(linux →
//     keystore_linux.go,android → keystore_android.go,其余 →
//     keystore_other.go);本文件不注册任何后端,不改变现有行为。

package keystore

// androidKeystoreStub 把 core 的构造面钉在本编译单元里,防止主机
// (非 android)构建的静态分析把 android 后端符号当作死代码。
var androidKeystoreStub = newAndroidKeystoreWithBridge
