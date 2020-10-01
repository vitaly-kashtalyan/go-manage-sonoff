package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
)

type devices []struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Enable bool   `json:"enable"`
}

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": http.StatusText(http.StatusOK),
		})
	})

	r.GET("/devices", func(c *gin.Context) {
		c.JSON(http.StatusOK, getDevices())
	})

	r.POST("/device/:id/*proxyPath", proxy)

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func getDevices() devices {
	jsonFile, err := os.Open("devices.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	var d devices
	_ = json.Unmarshal(byteValue, &d)
	return d
}

func proxy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	remote, err := url.Parse("http://" + getHostById(id))
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func getHostById(id int) string {
	for _, device := range getDevices() {
		if device.Id == id {
			return device.Host
		}
	}
	return "127.0.0.1"
}
