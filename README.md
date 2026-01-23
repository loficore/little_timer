# 关于项目

该项目是一个基于Zig的定时器项目，虽然希望实现的功能也并非只是定时器，但是暂时也就定为这个目标了。

# 关于协议
本项目采用[Appache License 2.0](./LICENSE),请依照协议使用

# 关于构建

## 推荐方案：使用 Zig 自动编译

Zig 内置了 C 编译器，会自动编译 `zig-webui` 依赖中的 C 库。**不需要手动安装任何系统库**。

### Linux / WSL / Windows
```bash
zig build run
```

就这么简单！Zig 会自动：
1. 下载 `zig-webui` 依赖
2. 检测到 webui 的 C 源码
3. 用内置编译器编译成库
4. 链接到可执行文件

### 工作原理

```
build.zig
  ↓
b.dependency("webui", ...)  // 获取 zig-webui 依赖
  ↓
webui_dep.module("webui")   // 导入 webui 模块
  ↓
webui 模块的 build.zig 中已经包含：
  - 编译 C 库的逻辑
  - linkLibrary() 的链接声明
  ↓
exe.root_module.addImport("webui", ...)  // Zig 自动处理所有链接
```

**关键点**：我们的 `build.zig` **只需导入 webui 模块**，不需要手动调用 `linkSystemLibrary`。

---

## 旧方案（不推荐）：MinGW64 手动管理

如果你想手动管理库（不推荐），可以参考以下步骤。但通常不必要。

### Linux 手动安装（可选）

```bash
# 如果你想使用系统预装的 webui 库（而不是让 Zig 自动编译）
git clone https://github.com/webui-dev/webui.git
cd webui
make install  # 安装到 /usr/local/lib
```

### Windows MinGW64 手动安装（可选）

在 MinGW64 shell 中：

```bash
# 设置别名（可选）
alias zig='/c/Users/YOUR_USERNAME/AppData/Local/Microsoft/WinGet/Packages/.../zig.exe'

# 安装 pkg-config
pacman -S mingw-w64-x86_64-pkg-config

# 编译 webui（可选，Zig 会自动做这个）
git clone https://github.com/webui-dev/webui.git
cd webui
make install
```

---

## 常见问题

**Q：为什么我之前编译失败？**  
A：因为 `build.zig` 里使用了 `exe.linkSystemLibrary("webui")`，这告诉 Zig 去系统里找库文件。现在已改为让 Zig 自动编译。

**Q：Zig 怎么编译 C 代码的？**  
A：Zig 内置了基于 LLVM 的 C 编译器，可以跨平台编译 C 代码。

**Q：编译很慢？**  
A：第一次编译会编译 webui 的 C 源码，比较慢。之后会使用缓存，速度很快。可以用 `zig build-clean` 清理缓存。

**Q：我想用系统预装的 webui 库？**  
A：不推荐。使用 Zig 自动编译更可靠，跨平台兼容性更好。