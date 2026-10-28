# PackPilot Android

原生 Kotlin + Jetpack Compose 客户端，对接项目根目录的 PackPilot Go API。界面同时使用官方 Miuix 和 Material 3 组件，可在运行时切换。

## 功能

- 自定义服务器地址和页面密码登录，系统 CookieManager 管理会话。
- 密码不写入 SharedPreferences、数据库、日志或界面状态持久层。
- Docker/并行 worker 状态、任务搜索和状态筛选。
- 创建任务、查看实时日志、取消、重试、删除和发布到自建 AUR。
- 查看 PKGBUILD、`.SRCINFO`、完整构建日志和 Agent 日志。
- MD3 / Miuix 组件风格切换。
- 翡翠绿、海洋蓝、暮光紫、落日橙四套主题。
- 跟随系统、浅色、深色三种显示模式。
- 401 自动返回登录页，后台每 3 秒刷新任务。

## 构建环境

- JDK 17+，本机验证使用 JDK 21。
- Android SDK Platform 37。
- AGP 9.4.1、Gradle 9.6、Kotlin Compose 插件 2.4.21。
- Miuix `top.yukonga.miuix.kmp:miuix-ui-android:0.9.4`。

首次在 `android/` 目录创建 `local.properties`（已被 Git 忽略）：

```properties
sdk.dir=/你的/Android/Sdk
```

构建：

```bash
./gradlew assembleDebug
```

APK 输出：

```text
app/build/outputs/apk/debug/app-debug.apk
```

安装到已连接设备：

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

## 连接后端

- 标准 Android 模拟器：使用 `http://10.0.2.2:8080` 访问宿主机。
- 真机局域网：使用电脑局域网 IP，例如 `http://192.168.1.10:8080`，并确保防火墙允许该端口。
- 公网或生产环境：使用 HTTPS 域名，启用后端 `auth.cookie_secure: true`。
- 为便于本地调试，当前 Manifest 允许 cleartext HTTP；生产发布建议改为仅允许指定调试域名或全量 HTTPS。

API 规范：`http://服务器地址/api/openapi.yaml`；中文说明见根目录 `API.md`。

## 测试

```bash
./gradlew testDebugUnitTest
./gradlew lintDebug
./gradlew assembleDebug
```

测试包含真实本地 HTTP 请求、Cookie 会话、JSON 模型、纯文本下载、错误状态、服务器 URL 校验和主题预设。

还可让 Android API 客户端直接连接运行中的 Go 后端：

```bash
PACKPILOT_ANDROID_TEST_URL='http://127.0.0.1:8080' \
PACKPILOT_ANDROID_TEST_PASSWORD='测试页面密码' \
./gradlew testDebugUnitTest --tests 'com.packpilot.android.data.ApiClientTest.liveBackendWhenConfigured'
```

未提供这两个环境变量时，真实后端测试自动跳过，不影响常规单元测试。

## 主要结构

```text
app/src/main/java/com/packpilot/android/
├── MainActivity.kt          # Cookie、Compose 和主导航
├── MainViewModel.kt         # 认证、任务、文件和轮询状态
├── data/
│   ├── ApiClient.kt         # 类型安全 HTTP API
│   ├── Models.kt            # kotlinx.serialization 模型
│   └── SettingsStore.kt     # 非敏感服务器与外观偏好
└── ui/
    ├── Components.kt        # MD3/Miuix 自适应组件
    ├── Screens.kt           # 登录、任务、新建和设置
    └── Theme.kt             # 风格、预设和深色模式
```

当前没有连接 Android 设备的 CI 环境只能执行 JVM 测试、Lint 和 APK 构建；真机/模拟器交互验收需安装 APK 后另行执行。
