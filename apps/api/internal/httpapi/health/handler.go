package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
)

type Check func(context.Context) error

type Handler struct {
	mysqlCheck Check
	redisCheck Check
}

func NewHandler(mysqlCheck, redisCheck Check) Handler {
	return Handler{
		mysqlCheck: mysqlCheck,
		redisCheck: redisCheck,
	}
}

func (handler Handler) Live(ctx *gin.Context) {
	response.JSON(ctx, http.StatusOK, gin.H{"status": "ok"})
}

func (handler Handler) Ready(ctx *gin.Context) {
	checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 3*time.Second)
	defer cancel()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		results <- result{name: "mysql", err: handler.mysqlCheck(checkCtx)}
	}()
	go func() {
		defer waitGroup.Done()
		results <- result{name: "redis", err: handler.redisCheck(checkCtx)}
	}()
	go func() {
		waitGroup.Wait()
		close(results)
	}()

	dependencies := map[string]string{}
	ready := true
	for dependency := range results {
		if dependency.err != nil {
			dependencies[dependency.name] = "unavailable"
			ready = false
		} else {
			dependencies[dependency.name] = "ok"
		}
	}

	if !ready {
		response.Error(
			ctx,
			http.StatusServiceUnavailable,
			"DEPENDENCY_UNAVAILABLE",
			"服务依赖暂时不可用",
		)
		return
	}

	response.JSON(ctx, http.StatusOK, gin.H{
		"status":       "ok",
		"dependencies": dependencies,
	})
}
