# Gin Session 学习 Demo

这是一个学习 Go Web 会话管理的小项目，使用 Gin 和 `gin-contrib/sessions` 演示 **Cookie Session** 与 **Redis Session**。项目配有一个原生 HTML/JavaScript 页面，可以一边请求接口，一边通过浏览器开发者工具观察 Cookie 的写入、保存和发送过程。

主要学习目标：

- 理解 HTTP 无状态与 Session 如何关联多次请求。
- 对比把 Session 数据保存在 Cookie 和 Redis 中的区别。
- 观察响应头 `Set-Cookie` 与请求头 `Cookie` 的配合。
- 理解签名、加密、Cookie 属性和会话过期的作用。

当前默认运行 **Redis Session**。登录接口只保存账号，没有校验账号密码；密码不写入 Session。这是会话机制实验，不是完整的登录系统。

## 项目结构

```text
.
├── backend/
│   ├── main.go                         # 两种 Session 示例，默认启动 Redis 版本
│   ├── go.mod
│   └── doc/cookie-session-curl-hmac.md   # curl、Cookie 属性及 HMAC 的详细笔记
└── frontend/
    ├── index.html                      # 请求接口、登录和获取用户信息
    ├── server.mjs                      # 静态页面服务及接口代理
    └── package.json
```

| 模式 | 代码入口 | 接口 | Cookie 名称 | Session 数据 |
| --- | --- | --- | --- | --- |
| Redis Session（默认） | `main()` | `POST /login`、`POST /profile` | `sys-session` | Redis 中保存 `account` |
| Cookie Session | `main_old()` | `GET /hello` | `mySession` | Cookie 中保存 `login`、`uid`、`nickname` |

## 启动项目

### 1. 准备环境

- Go：当前 `backend/go.mod` 声明的版本是 **1.27.0**，请使用支持该版本的工具链。
- Node.js：**18 或更高版本**，前端不需要安装第三方依赖。
- Redis：默认模式需要 `127.0.0.1:6379`，不设置用户名和密码，使用 DB 0。

下面的命令均以项目根目录为起点；前后端分别在独立终端运行。

### 2. 启动 Redis

如果本机已安装 Redis，在单独终端运行：

```sh
redis-server --bind 127.0.0.1 --port 6379
```

如果已有符合上述配置的 Redis 服务，可以直接复用。检查连接：

```sh
redis-cli -h 127.0.0.1 -p 6379 ping
# 预期：PONG
```

Redis 地址、认证信息和签名密钥目前写在 `backend/main.go` 的 `redis.NewStore(...)` 中；需要其他配置时修改这里。第一个参数 `2` 表示连接池最大空闲连接数，**不是 Redis 数据库编号**。

### 3. 启动后端

```sh
cd backend
go mod download
go run .
```

后端监听 `http://localhost:8080`。Redis 不可连接时，默认入口会直接 panic，HTTP 服务不会启动。

### 4. 启动前端

另开终端，从项目根目录执行：

```sh
cd frontend
npm run dev
```

打开 **http://localhost:3000**。

前端服务器将 `/hello`、`/login`、`/profile` 代理到 `http://localhost:8080`，同时转发请求体、`Cookie` 和 `Set-Cookie`。浏览器请求的是前端同源地址，因此当前演示不需要配置后端 CORS。

可以调整前端端口和代理目标（macOS / Linux）：

```sh
PORT=3001 BACKEND_URL=http://localhost:8080 npm run dev
```

该配置只影响前端；后端监听端口仍由 Go 代码中的 `r.Run(":8080")` 决定。

### 5. 体验 Redis Session

1. 清除站点已有的 `sys-session` Cookie。
2. 点击「获取用户信息」：未登录时预期返回 `401`，提示「请先登录」。
3. 输入账号 `alice` 和任意非空密码，点击「登录」：预期返回 `Welcome alice!`。
4. 再点击「获取用户信息」：预期返回 `user profile: alice`。
5. 刷新页面后再次获取用户信息：只要 Cookie 和 Redis 记录仍有效，登录状态仍可恢复。
6. 删除 `sys-session` Cookie 后再次获取：预期重新返回 `401`。

前端表单要求填写账号和密码，但后端没有必填校验。直接调用接口传空账号可能得到登录成功响应，随后 `/profile` 仍会返回 `401`。

默认模式没有注册 `/hello`，所以页面上方的「请求 /hello」按钮会请求失败，这是两个示例入口不同造成的。

### 6. 切换到 Cookie Session

在 `backend/main.go` 中进行两处函数改名：

```go
// 原来的 Redis 示例 main() 改名
func main_redis() {
    // 保留原函数体
}

// 原来的 Cookie 示例 main_old() 改为启动入口
func main() {
    // 保留原函数体
}
```

停止并重新执行 `go run .`。此模式不依赖 Redis；前端无需修改，点击「请求 /hello」即可。此时 `/login` 和 `/profile` 不可用。

首次访问 `/hello` 会保存三个字段，响应为：

```json
{"data":"1:u00001superstart"}
```

后续请求读取 Cookie 中已有的数据。若要重新观察首次写入，删除 `mySession` Cookie 后再请求。

该版本 Cookie Store 默认带有 `Secure` 和 `SameSite=None`。如果本地 HTTP 测试时浏览器拒绝保存或发送 Cookie，可在 `cookie.NewStore(...)` 后、中间件注册前显式设置本地调试选项：

```go
store.Options(sessions.Options{
    Path:     "/",
    MaxAge:   86400 * 30,
    Secure:   false,
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
})
```

`main.go` 已导入 `net/http`。这是可选修改；HTTPS 部署时应启用 `Secure`。切回 Redis 模式时，将两个函数名恢复即可。

## 如何观察 Cookie

打开浏览器开发者工具，主要查看以下位置：

| 位置 | 观察内容 |
| --- | --- |
| Network → `/login` 或首次 `/hello` → Response Headers | `Set-Cookie`：服务器要求浏览器保存 Cookie |
| Application / Storage → Cookies | Cookie 名称、值、域、Path、过期时间和安全属性 |
| Network → 后续 `/profile` 或 `/hello` → Request Headers | `Cookie`：浏览器自动把符合规则的 Cookie 发回服务器 |

在默认 Redis 模式中，登录后可以执行：

```sh
redis-cli --scan --pattern 'session_*'
# 将下面的 session_实际ID 替换为扫描到的完整 key
redis-cli TTL 'session_实际ID'
redis-cli GET 'session_实际ID'
```

默认 Redis key 为 `session_<Session ID>`，内容经过 Gob 序列化，`GET` 的结果不一定是可读文本。Cookie 中的值是编码并签名后的 Session ID，不是完整 Redis key，也不包含 `account` 数据。

可以在独立测试 Redis 中删除对应记录，再携带原 Cookie 请求 `/profile`，观察「Cookie 仍存在，但会话已失效」的情况。

注意：

- 统一使用 `localhost` 访问；`localhost` 与 `127.0.0.1` 的 Cookie 不共享。
- Cookie 不按端口隔离；同源判断则包含端口。不要把「不同端口」等同于「Cookie 一定不共享」。
- `HttpOnly` 会阻止 `document.cookie` 读取，但开发者工具仍可查看，浏览器也仍能自动发送。
- 前端 JavaScript 不能通过 `response.headers.get('Set-Cookie')` 读取该响应头，应在 Network 面板中查看。

也可以跳过前端，直接用 curl 验证默认 Redis 模式：

```sh
# 不带 Cookie：预期 401
curl -i -X POST http://localhost:8080/profile

# 登录并保存 Cookie：预期 200
curl -i -c /tmp/gin-sessions-demo.cookies \
  -H 'Content-Type: application/json' \
  -d '{"account":"alice","password":"demo-password"}' \
  http://localhost:8080/login

# 携带 Cookie：预期 200
curl -i -X POST -b /tmp/gin-sessions-demo.cookies http://localhost:8080/profile
```

`-c` 保存响应中的 Cookie，`-b` 读取并发送 Cookie。curl 适合观察请求头和响应头，不能代替浏览器验证 SameSite 或 CORS 行为。

## Session 知识总结

### Cookie 与 Session 的关系

HTTP 本身无状态，每次请求不会自动记住之前的登录结果。Session 为多次请求提供会话状态；Cookie 是客户端保存并随请求发送少量数据的一种机制，经常用于传递会话凭据。

**Session 是会话管理方式，Cookie 是常用的载体。Session 数据既可以放在客户端 Cookie 中，也可以放在服务端存储中。**

本项目默认流程：

```text
浏览器提交 POST /login
    → Go 将 account 写入 Session
    → Save() 将数据保存到 Redis，并通过 Set-Cookie 返回会话凭据
    → 浏览器保存 sys-session

浏览器请求 POST /profile，并自动发送 Cookie
    → Go 验证 Cookie，恢复 Session ID
    → 读取 Redis 中对应的数据
    → 有有效 account：返回用户信息；否则返回 401
```

### Cookie Session 与 Redis Session 对比

| 维度 | Cookie Session | Redis Session |
| --- | --- | --- |
| 数据位置 | 客户端 Cookie | 服务端 Redis |
| Cookie 内容 | 编码、签名后的会话数据，可选加密 | 编码、签名后的 Session ID，可选加密 |
| 请求成本 | 每次携带完整会话数据，受 Cookie 大小限制（通常约 4 KB） | Cookie 较小，但需要访问 Redis |
| 多实例共享 | 实例使用相同密钥和配置即可验证 | 还需共享 Redis 存储 |
| 主动撤销 | 删除浏览器 Cookie 不会让已复制的有效 Cookie 失效，需要额外机制 | 可删除服务端记录使对应会话失效 |
| 主要依赖 | 密钥管理、Cookie 容量及有效期 | 密钥管理、Redis 可用性及记录有效期 |

### 签名不等于加密

本项目两个 Store 都只传入了第一个密钥，用于认证签名；没有传入第二个加密密钥。

- **编码**（例如 Base64）：改变数据表示方式，可逆，不提供保密性。
- **HMAC 签名**：服务器使用密钥校验数据是否被篡改，不隐藏内容。
- **加密**：配置第二个密钥后隐藏 Cookie 中的编码对象；不会自动加密 Redis 记录。

因此，Cookie Session 中的数据不能因为「看起来像乱码」就被认为是保密的。Redis 模式的账号不在 Cookie 中，但 Cookie 本身仍是会话凭据；复制一个仍有效的 Cookie 可能复用会话。

相关机制可参考 [gorilla/securecookie 官方说明](https://github.com/gorilla/securecookie)。

### Cookie 常见属性

| 属性 | 含义 |
| --- | --- |
| Domain | 可发送到哪些主机；不设置时限定当前主机 |
| Path | 可发送到哪些路径，例如 `/` |
| Max-Age / Expires | 相对存活秒数 / 绝对过期时间；同时设置时 Max-Age 优先 |
| HttpOnly | 限制 JavaScript 直接读取 Cookie |
| Secure | 通常仅通过 HTTPS 发送；部分客户端对 localhost 有开发例外 |
| SameSite | 控制跨站发送；常见取值为 Strict、Lax、None，None 需要同时设置 Secure |

浏览器根据这些属性决定是否发送 Cookie，请求头不会把所有属性一起回传。完整规则见 [MDN：Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)。

### Save 与过期

- `session.Set(...)` 修改当前请求中的会话数据，调用 `session.Save()` 才会持久化；应检查保存错误，并在写响应前保存。
- Cookie 过期与 Redis 记录过期是两个层次；任一环节失效都可能导致无法恢复登录状态。
- 当前 Redis Store 默认 Cookie 有效期和 Redis TTL 均为 30 天；`/profile` 只读取、不保存，因此访问它不会自动续期。
- 修改过期时间时，应核对 Cookie 有效期、签名校验期限和 Redis TTL，不能只改其中一项就假设全部同步。
- 当前版本 Redis Store 的 `MaxAge <= 0` 保存逻辑会删除记录；不要照搬其他 Store 的行为，用 `MaxAge: 0` 实现会话 Cookie。

### 从 Demo 到实际应用

实际登录流程还需要校验账号密码、登录后更新会话标识以防会话固定、提供退出及会话撤销机制，并结合业务处理 CSRF 防护。密钥应使用服务端妥善保管的随机值；项目中的硬编码密钥仅用于学习。

本项目暂未实现这些功能，也未显式统一两种 Store 的 Cookie 安全选项。学习时应区分「库的默认行为」与「业务主动配置」。

更详细的版本默认值、HMAC 编码过程与 Cookie 属性说明，见 [Session 实验笔记](backend/doc/cookie-session-curl-hmac.md)。笔记中的后端启动命令请在 `backend/` 目录执行。
