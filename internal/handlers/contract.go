package handlers

import "github.com/gin-gonic/gin"

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type handlerV1 interface {
	Init(router gin.IRouter)
}
