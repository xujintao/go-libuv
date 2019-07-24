package main

import (
	"log"

	"github.com/xujintao/go-libuv/net/http"
	"github.com/xujintao/go-libuv/poll"
)

// 照这样看来一个仓库就是一个模块，这个模块里可能有多个包
// 如果包引用了第三方模块的包，那么当前模块就要依赖第三方模块，就要依赖第三方仓库

// 例1，当前模块的包依赖第三方模块的包，需要modules的协助
// 例2，当前模块的包依赖自己模块的其他包，也需要modules的协助
// 例3，当前模块的包没有多余的依赖，则不需要modules的协助
// 总结，只要包有依赖，就需要包管理

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := http.ListenAndServe(":8080"); err != nil {
		log.Print(err)
		return
	}
	poll.Wait()
}
