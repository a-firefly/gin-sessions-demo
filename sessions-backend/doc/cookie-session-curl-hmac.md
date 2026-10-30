# Gin Redis Session：curl 请求、Cookie 属性与 HMAC 签名

更新日期：2026-10-30。本文基于本项目当前代码与依赖版本：

- `github.com/gin-contrib/sessions v1.1.2`
- `github.com/boj/redistore v1.4.2`
- `github.com/gorilla/sessions v1.4.0`
- `github.com/gorilla/securecookie v1.1.2`

文中“库默认值”特指上述版本；升级依赖后应重新核对。

## 1. 当前接口做了什么

当前执行的是 [`main.go`](../main.go) 中的 `main()`，使用 Redis 存储 Session：

```go
store, err := redis.NewStore(2, "tcp", "127.0.0.1:6379", "", "", []byte("local-demo-session-signing-key-32"))
if err != nil {
    panic("failed to connet to redis")
}
r.Use(sessions.Sessions("sys-session", store))
```

`NewStore` 的参数依次是连接池最大空闲连接数、网络、Redis 地址、用户名、密码和签名密钥。当前用户名与密码为空，使用 Redis DB 0；`2` 不是数据库编号。

| 接口 | 请求参数 | 成功响应 | 失败响应 |
| --- | --- | --- | --- |
| `POST /login` | JSON：`account`、`password` | 200，JSON 字符串 `"Welcome alice!"` | 请求绑定失败返回 400；保存 Session 失败返回 500，`{"error":"登录失败，无法保存 Session"}` |
| `POST /profile` | 无请求体；携带登录 Cookie | 200，JSON 字符串 `"user profile: alice"` | Session 中账号缺失或为空时返回 401，`{"error":"请先登录"}` |

当前登录接口只绑定参数，把 `account` 写入 Session 并调用 `Save()`，**没有校验账号密码，也没有必填校验**。`password` 不参与后续处理，也不写入 Session。空账号可以得到登录成功响应，但访问 `/profile` 时会返回 401。

Cookie 名称是 `sys-session`。Redis 保存 Gob 序列化后的 Session 数据，例如 `account=alice`；Cookie 保存的是经过编码与签名的 Session ID，而不是账号数据，也不是直接存放完整 Redis key。Redis key 的默认格式是 `session_<Session ID>`。

登录保存成功后，服务器通过 `Set-Cookie` 返回 Cookie。访问 `/profile` 时，服务器验证 Cookie、恢复 Session ID，再读取 Redis 中的数据。该接口只读取，不调用 `Save()`，所以不会刷新 Redis TTL 或 Cookie 期限，也正常情况下不会返回新的 `Set-Cookie`。再次登录并成功保存时会刷新期限。

原 Cookie 存储示例保留在 `main_old()` 中，默认不会执行，也不会注册 `GET /hello`。该示例使用 `mySession` 和密钥 `secret`，写入 `login=1`、`uid=u00001`、`nickname=superstart`，响应是 `{"data":"1:u00001superstart"}`。若要运行它，需要调整入口；它的 Cookie 内容和默认属性与当前 Redis 方案不同。

## 2. 如何使用 curl

先确保 `127.0.0.1:6379` 有可连接、无需认证的 Redis。例如本地已安装 Redis 且该端口尚未占用时，在单独终端运行：

```bash
redis-server --bind 127.0.0.1 --port 6379
```

然后在项目根目录启动 Go 服务（本项目 `go.mod` 声明 Go 1.27.0）：

```bash
go run main.go
```

Redis 不可连接时，初始化会触发上面的 panic，HTTP 服务不会启动。在另一个终端按顺序执行：

```bash
# 未登录，不携带 Cookie：预期 401
curl -i -X POST http://localhost:8080/profile

# 登录，并把服务器返回的 Cookie 保存到文件：预期 200
curl -i -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"account":"alice","password":"demo-password"}' \
  http://localhost:8080/login

# 读取文件，携带此前保存的 Cookie：预期 200
curl -i -X POST -b cookies.txt http://localhost:8080/profile

# 读取已有 Cookie，同时把本次响应中的 Cookie 更新写回文件
curl -i -X POST -b cookies.txt -c cookies.txt http://localhost:8080/profile
```

`-d` 会使 curl 使用 POST；`/profile` 无请求体，显式使用 `-X POST`。直接执行 `curl http://localhost:8080/profile` 是 GET，不匹配当前路由。

登录成功的响应体：

```json
"Welcome alice!"
```

携带有效 Cookie 访问 `/profile` 的响应体：

```json
"user profile: alice"
```

| 参数 | 作用 |
| --- | --- |
| `-i` | 同时显示响应头和响应体 |
| `-c cookies.txt` | 请求结束后将 Cookie 保存到文件 |
| `-b cookies.txt` | 从文件读取 Cookie，用于此次请求 |

`-c` 不代表读取已有 Cookie；需要复用时使用 `-b`。`cookies.txt` 是客户端 Cookie 文件，不是服务端的 Session 数据库。对 `/profile` 同时使用 `-b`、`-c` 也不会让服务端自动续期。

后续请求通常发送的是：

```http
Cookie: sys-session=MTc5...
```

`Path`、`Expires`、`Secure` 等属性用于客户端管理 Cookie，不会随这个请求头一起回传。

## 3. 如何理解 Set-Cookie

当前 Redis 存储登录成功后的响应头格式如下（值与时间仅为示例）：

```http
Set-Cookie: sys-session=MTc5...; Path=/; Expires=Sun, 29 Nov 2026 03:01:13 GMT; Max-Age=2592000
```

含义是：保存名为 `sys-session` 的 Cookie，值为 `MTc5...`，其余部分是使用规则。

### 3.1 当前库的默认配置

当前 `redis.NewStore` 使用 `redistore v1.4.2` 的默认配置：

```go
Path:   "/",
MaxAge: 86400 * 30, // 30 天
```

`Secure`、`HttpOnly` 默认均为 `false`，`SameSite` 为零值，响应头不输出这三个属性。不能把旧 Cookie 存储的默认配置套用到 Redis 存储上：

| 属性 | 当前 `redis.NewStore` | `main_old()` 的 `cookie.NewStore` |
| --- | --- | --- |
| Cookie 名称（业务配置） | `sys-session` | `mySession` |
| Path | `/` | `/` |
| MaxAge | 30 天 | 30 天 |
| Secure | `false`，省略 | `true`，输出 `Secure` |
| HttpOnly | `false`，省略 | `false`，省略 |
| SameSite | 零值，省略 | `None`，输出 `SameSite=None` |

### 3.2 Path：哪些请求路径可以携带 Cookie

`Path=/` 表示站点下所有路径都匹配；仍需满足域名、Secure、SameSite 等其他条件。

| 配置 | 匹配示例 | 不匹配示例 |
| --- | --- | --- |
| `/` | `/login`、`/profile`、`/api/users` | — |
| `/api` | `/api`、`/api/users` | `/profile`、`/apix` |

当前库默认值为 `/`。如果响应头完全省略 `Path`，客户端根据设置 Cookie 的请求路径计算默认目录，例如 `/api/login` 对应 `/api`；当前 `/login` 对应 `/`。

Path 是发送范围规则，不是可靠的安全隔离机制。

### 3.3 Expires：绝对过期时间

```text
Expires=Sun, 29 Nov 2026 03:01:13 GMT
```

表示 Cookie 在这个绝对时间过期。换成北京时间是 **2026 年 11 月 29 日 11:01:13**。

当前库在 `MaxAge > 0` 时计算：

```text
Expires = 保存时的服务器时间 + MaxAge 秒
```

因此默认是保存后 30 天，而不是某个固定日期。

### 3.4 Max-Age：相对存活时间

```text
Max-Age=2592000
2592000 秒 = 30 天
```

表示从客户端接收 Cookie 开始的存活秒数。当前库默认是 `2592000`。同时存在 `Expires` 和 `Max-Age` 时，客户端优先使用 `Max-Age`。

特别注意 Go 配置和响应头值的区别：

| Go 中 `Options.MaxAge` | 库输出的行为 |
| --- | --- |
| 大于 0 | 输出有效秒数，同时生成未来的 `Expires` |
| 等于 0 | 省略这两个过期属性，作为会话 Cookie |
| 小于 0 | 输出 `Max-Age=0` 和过去的 `Expires`，使 Cookie 过期 |

**Go 配置 `MaxAge: 0` 不等于响应头 `Max-Age=0`。** 上表描述 Cookie 属性生成规则；当前 `redistore v1.4.2` 的 `Save()` 在 `MaxAge <= 0` 时会删除 Redis 记录并返回空值 Cookie，因此不能用 `MaxAge: 0` 实现 Redis 会话 Cookie。删除登录状态应使用负值并保存。

未指定这两个属性时，会话 Cookie 的生命周期由客户端的会话管理决定；不能简单保证“关闭一个标签页就删除”，浏览器的会话恢复也可能保留它。

此外，Cookie 的客户端过期与服务端接受期限是两个层次。此库还会验证编码值里的时间戳，默认接受期限为 30 天。只在 `store.Options(...)` 中改 `MaxAge`，不会同步修改底层 securecookie 的时间戳验证期限；Redis 存储会在下次保存时将正值 `MaxAge` 用作 `SETEX` 的 TTL。当前默认 Cookie 期限、签名接受期限、Redis TTL 都是 30 天，读取 `/profile` 不刷新这些期限。即使 Cookie 仍有效，Redis 记录已过期或被删除，也无法恢复账号。

需要同步调整 Redis 存储的 Cookie MaxAge 与签名验证期限时，可通过 `redis.GetRedisStore(store)` 取得底层存储，检查错误后调用其 `SetMaxAge(秒数)`，并使用正值。

### 3.5 Secure：限制通过安全连接发送

`Secure` 是标志属性，通过“出现或省略”表达，不使用 `=true` 或 `=false`。

| Go 配置 | 响应头 | 含义 |
| --- | --- | --- |
| `Secure: true` | 出现 `Secure` | 通常只通过 HTTPS 发送 |
| `Secure: false` | 省略该属性 | 允许通过 HTTP 或 HTTPS 发送 |

当前 Redis 存储默认是 `false`，允许本地 HTTP 调试。旧 Cookie 存储默认是 `true`；部分客户端对 `localhost` 有本地开发例外，因此带 Secure 的 Cookie 在 HTTP localhost 能工作，不代表部署到普通 HTTP 域名也能工作。

Secure 约束传输，不代表 Cookie 值本身已加密。

### 3.6 SameSite：跨站请求是否携带 Cookie

| 值 | 行为 |
| --- | --- |
| `Strict` | 仅同站请求携带；从其他站点点击链接进入时也不携带 |
| `Lax` | 同站请求携带；跨站顶层页面的安全导航，例如点击链接发起 GET，也可以携带 |
| `None` | 允许跨站携带，必须同时设置 `Secure` |

当前 Redis 存储省略 SameSite；旧 Cookie 存储默认是 `None`，同时设置了 `Secure`。

省略 SameSite 时，现代浏览器通常按 Lax 处理，部分浏览器有兼容性例外。`None` 也不能绕过浏览器的第三方 Cookie 限制。

“同站”通常根据协议和可注册域判断，“同源”还要求主机、端口一致。例如 `https://app.example.com` 和 `https://api.example.com` 通常同站但不同源。SameSite 和 CORS 分别解决不同问题。

curl 没有浏览器的页面导航上下文，不能用它验证浏览器的 SameSite 行为。

### 3.7 HttpOnly：限制 JavaScript 读取

`HttpOnly` 同样是标志属性。设置后，页面 JavaScript 无法通过 `document.cookie` 读取该 Cookie；浏览器仍会在满足规则的请求中自动携带它。

它与 Secure 相互独立：HttpOnly 限制脚本访问，Secure 限制传输方式。当前响应没有 HttpOnly，是因为默认没有启用。

## 4. sys-session 的值如何生成

当前 `redis.NewStore(...)` 只传入签名密钥 `local-demo-session-signing-key-32`。第一个密钥用于认证，也就是生成和验证 HMAC；第二个密钥可选，用于加密。当前 Cookie 中的 Session ID **有认证保护，但没有加密**。

首次保存时，库生成 32 字节随机数据，转为无填充的 Base32 字符串作为 Session ID。Session 数据单独 Gob 序列化后，通过 `SETEX session_<Session ID> 2592000 <数据>` 写入 Redis。Cookie 的编码对象仅为 Session ID。

完整生成流程如下：

```text
Session ID 字符串（不是 account 数据）
    ↓ Gob 序列化
    ↓ 可选：AES 加密（当前未启用）
    ↓ URL 安全的 Base64 编码，得到 payload
    ↓ 拼接 sys-session|时间戳|payload
    ↓ 使用签名密钥计算 HMAC-SHA256，得到 MAC
    ↓ 拼接 时间戳|payload|MAC原始字节
    ↓ 整体进行 URL 安全的 Base64 编码
最终 Cookie 值
```

Base64 是编码，不提供保密性；没有加密时，知道格式的人可以还原 Session ID，但不能从当前 Cookie 中解出 Redis 中的账号数据。旧 Cookie 存储方案的编码对象是完整 Session 数据，所以可以还原其中的 `login`、`uid`、`nickname`。

Cookie 名称参与签名，但不包含在最终编码值内部，因为读取 Cookie 时服务器已经知道名称。

## 5. HMAC 具体怎么使用签名密钥

### 5.1 参与计算的消息

库使用 HMAC-SHA256，消息格式是：

```text
sys-session|时间戳|payload
```

时间戳是生成时的 Unix 秒数。下面的代码展示核心计算；`payload` 代表前一步得到的实际编码数据：

```go
import (
    "crypto/hmac"
    "crypto/sha256"
)

message := "sys-session|" + timestamp + "|" + payload

h := hmac.New(sha256.New, []byte("local-demo-session-signing-key-32"))
h.Write([]byte(message))
mac := h.Sum(nil) // 32 字节的原始 MAC
```

这不是简单的 `SHA256(message + secret)`，也不是把签名密钥直接写进 Cookie。

最终编码的核心逻辑相当于：

```go
body := []byte(timestamp + "|" + payload + "|")
body = append(body, mac...)
cookieValue := base64.URLEncoding.EncodeToString(body)
```

注意 MAC 使用原始字节拼接，不是先转成十六进制字符串。

### 5.2 HMAC 内部计算

HMAC-SHA256 的标准结构：

```text
SHA256((K ⊕ opad) || SHA256((K ⊕ ipad) || message))
```

| 符号 | 含义 |
| --- | --- |
| `K` | 规范化成 64 字节的密钥 |
| `ipad` | 64 个 `0x36` 字节 |
| `opad` | 64 个 `0x5c` 字节 |
| `⊕` | 按位异或 |
| `||` | 字节拼接 |

对于 SHA256：密钥超过 64 字节时先做 SHA256；不足 64 字节时在末尾补零。当前演示密钥 `"local-demo-session-signing-key-32"` 实际是 33 字节，因此补零到 64 字节。

计算时先得到内层哈希，再得到外层哈希。这些步骤由 Go 的 `crypto/hmac` 完成，不需要自己实现。

相同密钥和相同消息得到相同 MAC。重新保存时，时间戳等消息内容可能改变，Cookie 值也可能改变。

### 5.3 服务器如何验证

客户端回传 Cookie 后，服务器依次：

1. 解开外层 Base64，取出时间戳、payload、MAC。
2. 用收到的数据重新拼接 `sys-session|时间戳|payload`。
3. 用服务端保存的签名密钥重新计算 HMAC-SHA256。
4. 使用恒定时间比较，检查两个 MAC 是否一致。
5. 验证时间戳是否在允许范围内。
6. 解开 payload 的 Base64；如启用了加密则解密，然后反序列化为 Session ID。
7. 使用 `session_<Session ID>` 查询 Redis，并反序列化恢复 Session 数据。

有人修改 Cookie 中编码的 Session ID 后，消息发生变化，原 MAC 无法通过验证。没有密钥，就无法为修改后的消息生成有效 MAC。

HMAC 防止未授权篡改，但不隐藏数据，也不阻止别人重放一个仍然有效的 Cookie。若密钥泄露，攻击者就可以生成有效 MAC。

## 6. 如何显式配置 Cookie 属性

以下是本地 HTTP 调试示例，放在 `NewStore` 之后、注册 Session 中间件之前：

```go
// 当前 main.go 已导入 "net/http"
store.Options(sessions.Options{
    Path:     "/",
    MaxAge:   86400 * 30,
    Secure:   false,
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
})
```

`store.Options(...)` 会替换 Cookie 选项，因此应显式写出需要保留的字段。本例仍使用 30 天，与底层默认验证期限一致。

HTTPS 环境使用 `Secure: true`；需要跨站 Cookie 时，根据实际业务设置 `SameSite=None` 并同时启用 Secure。

若需要加密 Cookie 中的 Session ID，可在创建 Redis 存储时传入第二个密钥（保留错误处理）：

```go
store, err := redis.NewStore(2, "tcp", "127.0.0.1:6379", "", "", authKey, encryptionKey)
if err != nil {
    panic(err)
}
```

- `authKey`：建议随机生成 32 或 64 字节，用于 HMAC。
- `encryptionKey`：必须是 16、24 或 32 字节，分别对应 AES-128、AES-192、AES-256。
- 密钥应在服务端妥善保存并保持稳定；多实例需要使用一致的密钥。
- 不要每次启动都生成新密钥，否则之前发出的 Cookie 无法继续验证。
- 当前硬编码签名密钥与旧示例的 `"secret"` 均仅用于演示；实际系统应使用妥善保存的随机密钥。

第二个密钥只加密 Cookie 的编码对象，不会加密 Redis 中保存的账号数据；当前 Redis 数据仍使用 Gob 序列化。

## 7. 复习速记

| 概念 | 一句话记忆 |
| --- | --- |
| `sys-session` | 当前 Cookie 名称；旧示例使用 `mySession` |
| Cookie 值 | 编码后的 Session ID、时间戳和 MAC |
| Redis 记录 | `session_<Session ID>` → Gob 序列化的账号数据，默认 TTL 为 30 天 |
| 签名密钥 | 服务端计算和验证 MAC，当前为演示用硬编码字符串 |
| Base64 | 编码，不能保密 |
| HMAC | 认证消息、防止未授权篡改，不提供加密 |
| AES | 配置第二个密钥后用于加密 Cookie 中的 Session ID |
| `Path` | 哪些路径可以携带 Cookie |
| `Expires` | 绝对过期时间 |
| `Max-Age` | 相对存活秒数，优先于 Expires |
| `Secure` | 通常只通过 HTTPS 发送 |
| `HttpOnly` | 禁止 JavaScript 直接读取 Cookie |
| `SameSite` | 控制跨站请求是否携带 Cookie |
| `curl -c` | 保存 Cookie |
| `curl -b` | 读取并携带 Cookie |

## 8. 参考资料

当前行为已对照本地依赖源码中的构造函数、`Save`、`load` 和 securecookie 编解码实现核对。

- [gin-contrib/sessions v1.1.2：Redis 封装与配置](https://github.com/gin-contrib/sessions/blob/v1.1.2/redis/redis.go)
- [redistore v1.4.2：默认选项、Session ID、Redis TTL 与保存逻辑](https://github.com/boj/redistore/blob/v1.4.2/redistore.go)
- [gorilla/sessions v1.4.0：默认配置、保存与校验期限](https://github.com/gorilla/sessions/blob/v1.4.0/store.go)
- [gorilla/sessions v1.4.0：Expires 生成逻辑](https://github.com/gorilla/sessions/blob/v1.4.0/sessions.go)
- [gorilla/securecookie v1.1.2：编码、HMAC 和验证实现](https://github.com/gorilla/securecookie/blob/v1.1.2/securecookie.go)
- [RFC 6265：HTTP Cookie、Path、Expires、Max-Age、Secure](https://httpwg.org/specs/rfc6265.html)
- [HTTP Cookie 规范草案：SameSite 等规则（持续更新）](https://httpwg.org/http-extensions/draft-ietf-httpbis-rfc6265bis.html)
