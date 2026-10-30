package main

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
)

// 演示把 session 中的数据存储到浏览器的 cookie 中
func main_old() {
	println("hello, world!")
	r := gin.Default()
	// 可传递两个 item， 第一个 key 用于 authentication， 第二个 key(可省略) 用于 encryption。
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mySession", store))

	r.GET("/hello", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		if session.Get("login") != "1" {
			session.Set("login", "1")
			session.Set("uid", "u00001")
			session.Set("nickname", "superstart")
			session.Save()
		}

		uid := session.Get("uid")
		login := session.Get("login")
		nickname := session.Get("nickname")

		data := fmt.Sprintf("%s:%s%s", login, uid, nickname)

		ctx.JSON(200, gin.H{ "data": data})
	})

	r.Run(":8080")
}

// 演示把 session 中的数据存储到Redis 中，给前端 cookie 中只存储数据在 Redis 中的 key
func main() {
	r := gin.Default()

	store, err := redis.NewStore(2, "tcp", "127.0.0.1:6379", "", "", []byte("local-demo-session-signing-key-32"))
	if err != nil {
		panic("failed to connet to redis")
	}

	r.Use(sessions.Sessions("sys-session", store))

	r.POST("/login", func(ctx *gin.Context) {
		var data struct {
			Account string `json:"account"`
			Password string `json:"password"`
		}

		if err := ctx.Bind(&data); err != nil {
			return
		}

		session := sessions.Default(ctx)
		session.Set("account", data.Account)
		if err := session.Save(); err != nil {
			fmt.Printf("failed to save session to redis\n")
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败，无法保存 Session"})
			return
		}

		account := session.Get("account")
		resp := "Welcome " + account.(string) + "!"
		ctx.JSON(http.StatusOK, resp)
	})


	r.POST("/profile", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		account, ok := session.Get("account").(string)
		if !ok || account == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		
		ctx.JSON(http.StatusOK, "user profile: " + account)
	})

	r.Run(":8080")
}
