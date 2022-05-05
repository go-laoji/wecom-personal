package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	wework "github.com/go-laoji/wecom-go-sdk"
	"github.com/go-laoji/wecom-go-sdk/config"
	"github.com/go-laoji/wecom-go-sdk/pkg/svr/logic"
	"github.com/go-laoji/wxbizmsgcrypt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var wecom wework.IWeWork

func Home(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "home"})
}

func DataGetHandler(c *gin.Context) {
	var params logic.EventPushQueryBinding
	if ok := c.ShouldBindQuery(&params); ok == nil {
		receiveId := params.CorpId
		if receiveId == "" {
			receiveId = wecom.GetCorpId()
			// TODO: 此处的receiveid其实为空的
			wxcpt := wxbizmsgcrypt.NewWXBizMsgCrypt(wecom.GetSuiteToken(), wecom.GetSuiteEncodingAesKey(),
				receiveId, wxbizmsgcrypt.XmlType)
			echoStr, cryptErr := wxcpt.VerifyURL(params.MsgSign, params.Timestamp, params.Nonce, params.EchoStr)
			if nil != cryptErr {
				wecom.Logger().Sugar().Error(cryptErr)
				c.JSON(http.StatusLocked, gin.H{"err": cryptErr, "echoStr": echoStr})
			} else {
				c.Writer.Write(echoStr)
			}
		}
	}
}

func DataPostHandler(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"errno": 400, "errmsg": err.Error()})
	} else {
		fmt.Println(string(body))
		c.Writer.WriteString("success")
	}
}

func ReceiverGetHandler(c *gin.Context) {
	var params logic.EventPushQueryBinding
	if ok := c.ShouldBindQuery(&params); ok == nil {
		receiveId := params.CorpId
		if receiveId == "" {
			receiveId = wecom.GetCorpId()
			// TODO: 此处的receiveid其实为空的
			wxcpt := wxbizmsgcrypt.NewWXBizMsgCrypt(wecom.GetSuiteToken(), wecom.GetSuiteEncodingAesKey(),
				receiveId, wxbizmsgcrypt.XmlType)
			echoStr, cryptErr := wxcpt.VerifyURL(params.MsgSign, params.Timestamp, params.Nonce, params.EchoStr)
			if nil != cryptErr {
				wecom.Logger().Sugar().Error(cryptErr)
				c.JSON(http.StatusLocked, gin.H{"err": cryptErr, "echoStr": echoStr})
			} else {
				c.Writer.Write(echoStr)
			}
		}
	}
}

func ReceiverPostHandler(c *gin.Context) {
	var params logic.EventPushQueryBinding
	if ok := c.ShouldBindQuery(&params); ok == nil {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			wecom.Logger().Sugar().Error(err)
			c.JSON(http.StatusOK, gin.H{"errno": 500, "errmsg": err.Error()})
			return
		} else {
			//　TODO:　此片的receiveid　就是suiteid
			wxcpt := wxbizmsgcrypt.NewWXBizMsgCrypt(wecom.GetSuiteToken(), wecom.GetSuiteEncodingAesKey(),
				wecom.GetSuiteId(), wxbizmsgcrypt.XmlType)
			if msg, err := wxcpt.DecryptMsg(params.MsgSign, params.Timestamp, params.Nonce, body); err != nil {
				wecom.Logger().Sugar().Error(err)
				c.JSON(http.StatusOK, gin.H{"errno": 500, "errmsg": err.ErrMsg})
				return
			} else {
				wecom.Logger().Sugar().Info(string(msg))
				var bizEvent logic.BizEvent
				if e := xml.Unmarshal(msg, &bizEvent); e != nil {
					wecom.Logger().Sugar().Error(e)
					c.JSON(http.StatusOK, gin.H{"errno": 500, "errmsg": err.ErrMsg})
					return
				}
				switch bizEvent.InfoType {
				case logic.SuiteTicket:
					wecom.Logger().Sugar().Info(string(msg))
					go logic.SuiteTicketEventLogic(msg, wecom)
					break
				case logic.CreateAuth:
					// TODO: 解密事件推送内容
					wecom.Logger().Sugar().Info(string(msg))
					break
				case logic.CancelAuth:
					go logic.CancelAuthEventLogic(msg, wecom)
					break
				case logic.ResetPermanentCode:
					go logic.ResetPermanentCodeEventLogic(msg, wecom)
					break
				}
				c.Writer.WriteString("success")
			}
		}
	} else {
		wecom.Logger().Sugar().Error(ok)
		c.JSON(http.StatusOK, gin.H{"errno": 400, "errmsg": ok.Error()})
	}
}

func main() {
	router := gin.Default()
	router.GET("/home", Home)
	c := config.ParseFile("personal.yml")
	logic.Migrate(c.Dsn)
	wecom = wework.NewWeWork(wework.WeWorkConfig{
		SuiteId:             c.SuiteId,
		SuiteSecret:         c.SuiteSecret,
		SuiteToken:          c.SuiteToken,
		SuiteEncodingAesKey: c.SuiteEncodingAesKey,
	})
	callbackGroup := router.Group("callback")
	{
		callbackGroup.GET("/data", DataGetHandler)
		callbackGroup.POST("/data", DataPostHandler)
		callbackGroup.GET("/receiver", ReceiverGetHandler)
		callbackGroup.POST("/receiver", ReceiverPostHandler)
	}
	srv01 := &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%v", c.Port),
		Handler:        router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		if err := srv01.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv01.Shutdown(ctx); err != nil {
		log.Fatal("server shutdown:", err)
	}
}
