# 最简前端

需要 Node.js 18 或更高版本，无需安装第三方依赖。

先在后端目录启动接口：

```sh
cd ../seesions-backend
go run .
```

另开一个终端启动前端：

```sh
cd ../sessions-frontend
npm run dev
```

打开 http://localhost:3000：

- 上方保留 `GET /hello` 的请求按钮。
- 下方输入账号和密码，点击「登录」，提交 JSON 到 `POST /login`。
- 登录成功后，点击「获取用户信息」，请求 `POST /profile`。

当前后端 `main()` 需要本地 Redis（`127.0.0.1:6379`），只注册了 `/login` 和 `/profile`；`/hello` 在 `main_old()` 中，当前启动入口访问它会返回 404。

当前后端为演示实现：登录只保存账号，没有校验密码；未登录时请求 `/profile` 会返回 401，页面会显示错误。

后端使用 8080 端口，因此前端默认使用 3000。前端开发服务器将 `/hello`、`/login`、`/profile` 代理到 `http://localhost:8080`，并转发请求体和 Session Cookie，无需修改后端跨域配置。

可选配置（macOS / Linux）：

```sh
PORT=3001 BACKEND_URL=http://localhost:8080 npm run dev
```
